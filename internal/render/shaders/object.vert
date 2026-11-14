#version 330
// objectVS is meshVS that can also draw instances (DrawMeshInstanced),
// each with its own transform.
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
in mat4 instanceTransform;
uniform mat4 matModel;
uniform mat4 matNormal;
uniform float instanced;
// A detail flush on a bigger piece is drawn a layer above it (mesh.Unfight):
// the layer comes in the length of the normal, 1 + layer, and brings the
// depth forward by eight steps of the depth buffer each, so the two
// surfaces don't fight. Other meshes have unit normals: layer 0.
const float layerStep = 1.0 / 1048576.0; // 2^-20 of the NDC depth
uniform vec3 origin;
// viewProj is the camera about origin, computed in double precision (relVP):
// projecting fragRel rather than the world position keeps the quarter
// millimeter of float grain off the screen, where it made close surfaces
// fight for each pixel as the camera moved.
uniform vec4 viewProj[4];
out vec3 fragRel;
out vec3 fragPos;
out vec3 fragNormal;
out vec3 localPos;
out vec3 localNormal;
out vec2 fragTexCoord;
void main() {
    float layer = floor(length(vertexNormal) + 0.5) - 1.0;
    fragTexCoord = vertexTexCoord;
    localPos = vertexPosition;
    localNormal = vertexNormal;
    if (instanced > 0.5) {
        vec4 p = instanceTransform * vec4(vertexPosition, 1.0);
        fragPos = p.xyz;
        fragRel = fragPos - origin;
        fragNormal = normalize(transpose(inverse(mat3(instanceTransform))) * vertexNormal);
        gl_Position = mat4(viewProj[0], viewProj[1], viewProj[2], viewProj[3]) * vec4(fragRel, 1.0);
        gl_Position.z -= layer * layerStep * gl_Position.w;
        return;
    }
    fragPos = vec3(matModel * vec4(vertexPosition, 1.0));
    fragRel = fragPos - origin;
    fragNormal = normalize(vec3(matNormal * vec4(vertexNormal, 0.0)));
    gl_Position = mat4(viewProj[0], viewProj[1], viewProj[2], viewProj[3]) * vec4(fragRel, 1.0);
    gl_Position.z -= layer * layerStep * gl_Position.w;
}
