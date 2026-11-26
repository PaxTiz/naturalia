#version 330
// bloom_down.frag halves the image with a 13-tap filter that avoids flicker.
// The first pass keeps only what is brighter than white, with a soft knee.
in vec2 fragTexCoord;
uniform sampler2D texture0;
uniform vec2 texel;
uniform float first;
out vec4 finalColor;
vec3 s(vec2 o) { return texture(texture0, fragTexCoord + o * texel).rgb; }
void main() {
    vec3 c = s(vec2(0)) * 0.125;
    c += (s(vec2(-1, -1)) + s(vec2(1, -1)) + s(vec2(-1, 1)) + s(vec2(1, 1))) * 0.125;
    c += (s(vec2(-2, -2)) + s(vec2(2, -2)) + s(vec2(-2, 2)) + s(vec2(2, 2))) * 0.03125;
    c += (s(vec2(-2, 0)) + s(vec2(2, 0)) + s(vec2(0, -2)) + s(vec2(0, 2))) * 0.0625;
    if (first > 0.5) {
        float b = max(c.r, max(c.g, c.b));
        float knee = clamp(b - 0.6, 0.0, 0.8);
        float w = max(knee * knee / 3.2, b - 1.0) / max(b, 1e-4);
        c *= w;
    }
    finalColor = vec4(c, 1.0);
}
