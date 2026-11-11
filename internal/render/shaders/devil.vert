#version 330
// devil.vert bends a flat grid into the whirlwind: u goes around, v goes up.
// The column narrows above a dusty skirt, flares at the top and sways.
in vec3 vertexPosition;
in vec2 vertexTexCoord;
uniform mat4 mvp;
uniform mat4 matModel;
uniform float time;
uniform vec4 devil; // x: seed, y: strength
uniform vec3 origin;
out vec3 fragRel;
out vec3 fragPos;
out vec3 fragNormal;
out vec2 tube;
void main() {
    float a = vertexTexCoord.x * 6.2831853;
    float t = vertexTexCoord.y;
    float r = 0.45 + 0.55 * pow(t, 1.3) + 0.7 * exp(-t * 18.0);
    vec2 sway = vec2(sin(t * 3.0 + time * 0.7 + devil.x), cos(t * 2.3 + time * 0.5 + devil.x * 1.7)) * 0.8 * t * t;
    vec3 p = vec3(cos(a) * r + sway.x, t, sin(a) * r + sway.y);
    tube = vec2(a, t);
    fragPos = vec3(matModel * vec4(p, 1.0));
    fragRel = fragPos - origin;
    fragNormal = normalize(mat3(matModel) * vec3(cos(a), 0.0, sin(a)));
    gl_Position = mvp * vec4(p, 1.0);
}
