#version 330
// terrainVS decodes the light baked in the length of the normals (see
// Terrain.light).
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
uniform mat4 mvp;
uniform vec3 origin;
uniform vec4 lightVP[4]; // the shadow map's camera about origin, by columns
out vec3 fragPos;
out vec3 fragRel;
out vec3 shadowCoord; // where the vertex falls in the shadow map
out vec3 fragNormal;
out vec2 fragTexCoord;
out float fragSun;
out float fragAO;
void main() {
    float l = length(vertexNormal) - 1.0;
    float occ = floor(l + 0.01);
    fragSun = clamp((l - occ) / 0.9, 0.0, 1.0);
    fragAO = occ / 31.0;
    fragTexCoord = vertexTexCoord;
    fragPos = vertexPosition; // terrain meshes are drawn with no transform
    fragRel = vertexPosition - origin;
    vec4 lp = mat4(lightVP[0], lightVP[1], lightVP[2], lightVP[3]) * vec4(fragRel, 1.0);
    shadowCoord = lp.xyz / lp.w * 0.5 + 0.5; // orthographic: linear, so exact per vertex
    fragNormal = normalize(vertexNormal);
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}
