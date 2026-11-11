#version 330
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
uniform mat4 mvp;
uniform mat4 matModel;
uniform mat4 matNormal;
out vec3 fragPos;
out vec3 fragNormal;
out vec3 localPos;
out vec3 localNormal;
out vec2 fragTexCoord;
out vec3 fragRel;
uniform vec3 origin;
void main() {
    fragTexCoord = vertexTexCoord;
    localPos = vertexPosition;
    localNormal = vertexNormal;
    fragPos = vec3(matModel * vec4(vertexPosition, 1.0));
    fragRel = fragPos - origin;
    fragNormal = normalize(vec3(matNormal * vec4(vertexNormal, 0.0)));
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}
