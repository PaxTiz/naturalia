#version 330
// bloom_up.frag blurs a level with a tent filter as it is added to the one above.
in vec2 fragTexCoord;
uniform sampler2D texture0;
out vec4 finalColor;
void main() {
    vec2 t = 1.0 / vec2(textureSize(texture0, 0));
    vec3 c = texture(texture0, fragTexCoord).rgb * 4.0;
    c += (texture(texture0, fragTexCoord + vec2(t.x, 0)).rgb + texture(texture0, fragTexCoord - vec2(t.x, 0)).rgb
        + texture(texture0, fragTexCoord + vec2(0, t.y)).rgb + texture(texture0, fragTexCoord - vec2(0, t.y)).rgb) * 2.0;
    c += texture(texture0, fragTexCoord + t).rgb + texture(texture0, fragTexCoord - t).rgb
        + texture(texture0, fragTexCoord + vec2(t.x, -t.y)).rgb + texture(texture0, fragTexCoord - vec2(t.x, -t.y)).rgb;
    finalColor = vec4(c / 16.0, 1.0);
}
