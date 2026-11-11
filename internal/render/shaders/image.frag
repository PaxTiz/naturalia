#version 330
// image.frag makes the final image. image() is the whole look for one pixel;
// FXAA then blends along the edges it finds in the result.
in vec2 fragTexCoord;
uniform sampler2D texture0; // HDR scene
uniform sampler2D bloom;
uniform float exposure;
uniform float sunUp;        // sun height where the player stands
uniform float time;
uniform vec3 sunScreen;     // xy: the sun on screen (uv), z: light shafts strength
out vec4 finalColor;

// Light shafts: the bright parts (the bloom's source) smeared toward the
// sun, so they stream out from behind the relief when the sun is low.
vec3 shafts(vec2 uv) {
    if (sunScreen.z <= 0.0) return vec3(0.0);
    vec2 stride = (sunScreen.xy - uv) / 32.0;
    vec3 sum = vec3(0.0);
    float w = 1.0;
    for (int i = 0; i < 32; i++) {
        uv += stride;
        sum += texture(bloom, uv).rgb * w;
        w *= 0.92;
    }
    return sum / 32.0 * sunScreen.z;
}

vec3 rays; // light shafts, computed once per pixel in main

vec3 image(vec2 uv) {
    vec3 c = texture(texture0, uv).rgb + texture(bloom, uv).rgb * ${bloomStrength} + rays;
    // Filmic curve (ACES, Narkowicz's fit): deeper shadows and a soft
    // shoulder. The 0.53 keeps the mid-tones where the old curve had them.
    c *= exposure * 0.53;
    c = clamp(c * (2.51 * c + 0.03) / (c * (2.43 * c + 0.59) + 0.14), 0.0, 1.0);

    // Grading by time of day: warm and rich in the day, cool shadows and
    // warm highlights in the blue dusk, cold and greyer at night.
    float day = smoothstep(-0.1, 0.25, sunUp);
    float dusk = smoothstep(-0.3, -0.02, sunUp) * (1.0 - smoothstep(0.02, 0.3, sunUp));
    float l = dot(c, vec3(0.2126, 0.7152, 0.0722));
    float sat = mix(0.82, 1.0, day);
    c = mix(vec3(l), c, sat);
    c *= mix(vec3(0.88, 0.95, 1.12), vec3(1.0), day);
    // Split toning, the style's warm light and cool shade: violet in the
    // shadows all day, stronger and bluer at dusk.
    // Light now: the lighting itself gives warm light and cool shade.
    vec3 shadows = mix(mix(vec3(1.0), vec3(0.98, 0.97, 1.03), day), vec3(0.94, 0.97, 1.07), dusk);
    vec3 lights = mix(mix(vec3(1.0), vec3(1.015, 1.0, 0.98), day), vec3(1.05, 1.0, 0.94), dusk);
    c *= mix(shadows, lights, smoothstep(0.1, 0.7, l));
    return clamp(c, 0.0, 1.0);
}

float luma(vec3 c) { return dot(c, vec3(0.299, 0.587, 0.114)); }

void main() {
    vec2 uv = fragTexCoord;
    rays = shafts(uv) * vec3(1.0, 0.75, 0.5);
    vec3 c = image(uv);
    vec2 q = uv - 0.5;
    c *= 1.0 - dot(q, q) * 0.45; // vignette
    finalColor = vec4(c, sqrt(luma(c))); // perceptual luma for FXAA
}
