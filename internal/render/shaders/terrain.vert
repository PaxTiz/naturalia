#version 330
// terrainVS decodes the light baked in the vertices (see Terrain.course):
// the occlusion in the length of the normals, and in the integer part of
// the texture coordinates the hour angles where the relief lets the sun
// through.
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
uniform vec3 origin;
// viewProj is the camera about origin, computed in double precision (relVP):
// projecting fragRel rather than the world position keeps the quarter
// millimeter of float grain off the screen, where it made close surfaces
// fight for each pixel as the camera moved.
uniform vec4 viewProj[4];
uniform vec3 viewAt; // the camera's eye, about origin: viewProj is about it (setViewProj)
uniform vec4 lightVP[4]; // the shadow map's camera about origin, by columns
uniform vec3 sunDir;
uniform vec3 sunAxis; // Mars' spin axis: the sun turns around it
out vec3 fragPos;
out vec3 fragRel;
out vec3 shadowCoord; // where the vertex falls in the shadow map
out vec3 fragNormal;
out vec2 fragTexCoord;
out float fragSun;
out float fragAO;
void main() {
    fragAO = clamp(length(vertexNormal) - 1.0, 0.0, 1.0);
    vec2 code = floor(vertexTexCoord);
    fragTexCoord = (vertexTexCoord - code) / 0.99;
    // The sun's hour angle here: how far it has turned past the local noon.
    vec3 up = normalize(vertexPosition);
    vec3 u = up - sunAxis * dot(up, sunAxis), s = sunDir - sunAxis * dot(sunDir, sunAxis);
    float h = atan(dot(sunAxis, cross(u, s)), dot(u, s));
    float rise = code.x > 999.5 ? -4.0 : -code.x / 1000.0 * 3.14159265;
    float set = code.y > 999.5 ? 4.0 : code.y / 1000.0 * 3.14159265;
    const float w = 0.035; // 2°: the penumbra
    fragSun = smoothstep(rise - w, rise + w, h) * (1.0 - smoothstep(set - w, set + w, h)) * step(0.001, set - rise);
    fragPos = vertexPosition; // terrain meshes are drawn with no transform
    fragRel = vertexPosition - origin;
    vec4 lp = mat4(lightVP[0], lightVP[1], lightVP[2], lightVP[3]) * vec4(fragRel - viewAt, 1.0);
    shadowCoord = lp.xyz / lp.w * 0.5 + 0.5; // orthographic: linear, so exact per vertex
    fragNormal = normalize(vertexNormal);
    gl_Position = mat4(viewProj[0], viewProj[1], viewProj[2], viewProj[3]) * vec4(fragRel - viewAt, 1.0);
}
