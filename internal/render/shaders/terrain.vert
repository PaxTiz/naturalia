#version 330
// terrainVS decodes the light baked in the length of the normals (see
// Terrain.light).
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
uniform vec3 origin;
// viewProj is the camera about origin, computed in double precision (relVP):
// projecting fragRel rather than the world position keeps the quarter
// millimeter of float grain off the screen, where it made close surfaces
// fight for each pixel as the camera moved.
uniform vec4 viewProj[4];
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
    gl_Position = mat4(viewProj[0], viewProj[1], viewProj[2], viewProj[3]) * vec4(fragRel, 1.0);
}
