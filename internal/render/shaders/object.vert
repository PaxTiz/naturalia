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
    fragTexCoord = vertexTexCoord;
    localPos = vertexPosition;
    localNormal = vertexNormal;
    if (instanced > 0.5) {
        vec4 p = instanceTransform * vec4(vertexPosition, 1.0);
        fragPos = p.xyz;
        fragRel = fragPos - origin;
        fragNormal = normalize(transpose(inverse(mat3(instanceTransform))) * vertexNormal);
        gl_Position = mat4(viewProj[0], viewProj[1], viewProj[2], viewProj[3]) * vec4(fragRel, 1.0);
        return;
    }
    fragPos = vec3(matModel * vec4(vertexPosition, 1.0));
    fragRel = fragPos - origin;
    fragNormal = normalize(vec3(matNormal * vec4(vertexNormal, 0.0)));
    gl_Position = mat4(viewProj[0], viewProj[1], viewProj[2], viewProj[3]) * vec4(fragRel, 1.0);
}
