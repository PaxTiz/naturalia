#version 330
// devil.frag paints spiraling streaks of dust, denser where the tube is seen
// face-on (its middle) and fading at the bottom, the top and the edges.
in vec3 fragPos;
in vec3 fragNormal;
in vec2 tube;
uniform float time;
uniform vec4 devil;
out vec4 finalColor;
#include "common.glsl"
void main() {
    vec3 v = normalize(viewRel - fragRel);
    vec3 n = normalize(fragNormal);
    float t = tube.y;
    float a = tube.x - time * 3.0 - t * 5.0;
    float swirl = fbm3(vec3(cos(a) * 1.5, sin(a) * 1.5, t * 7.0 - time * 0.8 + devil.x));
    float streak = smoothstep(0.35, 0.75, swirl);
    float fade = smoothstep(0.0, 0.06, t) * (1.0 - smoothstep(0.5, 1.0, t));
    float dense = mix(1.7, 0.8, smoothstep(0.0, 0.35, t)); // thickest near the ground
    float alpha = clamp(pow(abs(dot(n, v)), 1.5) * (0.25 + 0.75 * streak) * fade * dense * devil.y * 0.55, 0.0, 0.85);

    vec3 up = normalize(fragPos);
    float lit = 0.55 + 0.45 * clamp(dot(n, sunDir) * 0.5 + 0.5, 0.0, 1.0);
    vec3 col = vec3(0.62, 0.40, 0.26) * (0.75 + 0.5 * streak) * (sunCol * 0.8 * lit * daylight(up) + skyLight(up, up));
    finalColor = vec4(finish(col, length(viewRel - fragRel)), alpha);
}
