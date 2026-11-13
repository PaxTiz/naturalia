#version 330
in vec3 fragPos;
in vec3 fragNormal;
uniform vec3 up; // the viewer's local up: the horizon follows them around Mars
uniform float time;
uniform float air; // 1 on the ground, 0 high up in creative flight: space
out vec4 finalColor;
#include "common.glsl"
// moon draws one of the two small moons: a lumpy gray rock, lit by the
// sun. It returns its color and sets cover where it hides the sky.
vec3 moon(vec3 d, vec3 dir, float radius, float seed, inout float cover) {
    float c = dot(d, dir);
    if (c < 0.995) return vec3(0.0);
    // Position on the disk, in units of its radius.
    vec3 t1 = normalize(cross(dir, vec3(0.0, 1.0, 0.0)));
    vec3 t2 = cross(t1, dir);
    vec2 q = vec2(dot(d, t1), dot(d, t2)) / radius;
    float a = atan(q.y, q.x);
    float edge = 1.0 + 0.16 * (vnoise3(vec3(cos(a), sin(a), seed) * 1.6) - 0.5) * 2.0;
    float r = length(q) / edge;
    float k = 1.0 - smoothstep(0.92, 1.0, r);
    if (k <= 0.0) return vec3(0.0);
    vec3 n = normalize(t1 * q.x + t2 * q.y - dir * sqrt(max(0.0, 1.0 - r * r)));
    // Lit three-quarters toward the viewer, so the moons read well in the sky.
    float lit = clamp((dot(n, normalize(sunDir - dir * 1.2)) + 0.15) / 1.15, 0.0, 1.0);
    float craters = vnoise3(vec3(q * 3.0, seed)) * 0.5 + vnoise3(vec3(q * 7.0, seed + 3.0)) * 0.25;
    vec3 col = vec3(0.55, 0.50, 0.46) * (0.6 + 0.6 * craters) * (0.08 + 1.1 * lit);
    cover = k;
    return col * k;
}

void main() {
    vec3 d = normalize(fragRel - viewRel);
    float h = dot(d, up);
    vec3 col = skyColor(d, up) * air;
    float day = daylight(up) * air;

    // Stars come out at night only, on a fine 3D grid of directions.
    vec3 p = d * 220.0;
    vec3 cell = floor(p);
    float r = hash13(cell);
    if (r > 0.97) {
        vec3 star = cell + 0.5 + (vec3(hash13(cell + 1.0), hash13(cell + 2.0), hash13(cell + 3.0)) - 0.5) * 0.6;
        float twinkle = 0.7 + 0.3 * sin(time * (1.0 + r * 4.0) + r * 50.0);
        float bright = (r - 0.97) / 0.03;
        col += vec3(0.9, 0.93, 1.0) * smoothstep(0.32, 0.0, length(p - star)) * bright * twinkle * max(smoothstep(-0.02, 0.12, h), 1.0 - air) * (1.0 - day);
    }

    // Phobos, then tiny Deimos: pale in the day sky, bright at night.
    float cover = 0.0;
    vec3 m = moon(d, phobosDir, 0.04, 1.0, cover);
    col = mix(col, m + col * (0.55 * day), cover);
    cover = 0.0;
    m = moon(d, deimosDir, 0.015, 7.0, cover);
    col = mix(col, m + col * (0.55 * day), cover);

    // The sun looks smaller from Mars, with a soft glow in the dusty air.
    float s = max(dot(d, sunDir), 0.0);
    col += sunCol * 0.74 * (smoothstep(0.99975, 0.99988, s) * 5.0 + pow(s, 900.0) * 1.2 + pow(s, 40.0) * 0.15);

    finalColor = vec4(col, 1.0);
}
