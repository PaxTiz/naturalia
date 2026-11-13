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
// The photo mode's settings (post.go's look): the depth of field...
uniform sampler2D depthTex; // the scene's depth buffer
uniform vec4 dof;           // x: focus (m, 0 at the screen's center), y: largest blur (px), zw: clip planes
// ...and the grading: filter, contrast, saturation, vignette.
uniform vec4 look;
out vec4 finalColor;

float viewDist(vec2 uv) {
    float z = texture(depthTex, uv).r * 2.0 - 1.0;
    return 2.0 * dof.z * dof.w / (dof.w + dof.z - z * (dof.w - dof.z));
}

// blurSize is a point's circle of confusion, in pixels, as through a lens:
// it follows |1/focus - 1/d|, so focusing far keeps the landscape sharp
// and only what is near blurs, while focusing near blurs all behind. At
// full blur, focusing at 3 m blurs the horizon all the way.
float blurSize(float d, float focus) {
    return min(abs(1.0 / focus - 1.0 / d) * 3.0 * dof.y, dof.y);
}

// sceneColor is the HDR scene, through the lens: each pixel gathers the
// samples around it, on a golden angle spiral, whose own blur reaches it
// (Dennis Gustafsson's single pass bokeh). Bright points become discs.
vec3 sceneColor(vec2 uv) {
    vec3 c = texture(texture0, uv).rgb;
    if (dof.y <= 0.0) return c;
    vec2 px = 1.0 / vec2(textureSize(texture0, 0));
    float focus = dof.x;
    if (focus <= 0.0) { // autofocus: the nearest of five points at the center
        focus = viewDist(vec2(0.5));
        for (int i = 0; i < 4; i++) {
            vec2 o = vec2(i < 2 ? (i == 0 ? -1.0 : 1.0) : 0.0, i >= 2 ? (i == 2 ? -1.0 : 1.0) : 0.0);
            focus = min(focus, viewDist(vec2(0.5) + o * px * 12.0));
        }
    }
    float d = viewDist(uv);
    float size = blurSize(d, focus);
    float total = 1.0;
    // About 100 samples whatever the blur: steps grow with it.
    float stride = max(0.5, dof.y * dof.y / 200.0);
    float r = stride;
    for (float a = 0.0; r < dof.y; a += 2.39996323) {
        vec2 tuv = uv + vec2(cos(a), sin(a)) * px * r;
        vec3 sc = texture(texture0, tuv).rgb;
        float sd = viewDist(tuv);
        float ss = blurSize(sd, focus);
        // What lies behind the pixel can't blur over it more than the
        // pixel itself is blurred: sharp subjects keep their edges.
        if (sd > d) ss = min(ss, size * 2.0);
        float m = smoothstep(r - stride, r + stride, ss);
        c += mix(c / total, sc, m);
        total += 1.0;
        r += stride / r;
    }
    return c / total;
}

// photoFilter grades the picture with the photo mode's looks (PhotoFilters).
vec3 photoFilter(vec3 c, float l) {
    int f = int(look.x + 0.5);
    if (f == 1) { // golden: warm light, warm shade
        c *= vec3(1.08, 1.0, 0.84);
        c += vec3(0.03, 0.012, 0.0) * (1.0 - l);
    } else if (f == 2) { // dusk: cold and quiet
        c = mix(vec3(l), c, 0.85) * vec3(0.88, 0.96, 1.14);
    } else if (f == 3) { // cinema: an S curve, teal shade, orange light
        c = mix(vec3(l), c, 0.9);
        c *= mix(vec3(0.9, 1.0, 1.08), vec3(1.07, 1.0, 0.9), smoothstep(0.15, 0.75, l));
        c = c * c * (3.0 - 2.0 * c);
    } else if (f == 4) { // black and white, through a red filter like on film
        float m = dot(c, vec3(0.55, 0.38, 0.07));
        c = vec3(smoothstep(-0.05, 1.02, m));
    } else if (f == 5) { // film: faded blacks, warm, a little washed out
        c = mix(vec3(l), c, 0.8) * vec3(1.04, 1.0, 0.92);
        c = c * 0.88 + vec3(0.07, 0.06, 0.07);
    }
    return c;
}

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
    vec3 c = sceneColor(uv) + texture(bloom, uv).rgb * ${bloomStrength} + rays;
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

    c = photoFilter(clamp(c, 0.0, 1.0), l);
    l = dot(c, vec3(0.2126, 0.7152, 0.0722));
    c = mix(vec3(l), c, 1.0 + look.z);
    c = (c - 0.5) * (1.0 + look.y * 0.6) + 0.5;
    return clamp(c, 0.0, 1.0);
}

float luma(vec3 c) { return dot(c, vec3(0.299, 0.587, 0.114)); }

void main() {
    vec2 uv = fragTexCoord;
    rays = shafts(uv) * vec3(1.0, 0.75, 0.5);
    vec3 c = image(uv);
    vec2 q = uv - 0.5;
    c *= max(0.0, 1.0 - dot(q, q) * look.w); // vignette
    finalColor = vec4(c, sqrt(luma(c))); // perceptual luma for FXAA
}
