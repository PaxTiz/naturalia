#version 330
// holoFS paints the hologram of Mars above the map table: a glowing
// grid, the cleaned areas, the base and the player. Drawn additively.
in vec3 fragPos;
in vec3 fragNormal;
uniform sampler2D texture0; // cleaned fraction of each dust tile
uniform vec3 viewPos;
uniform vec3 origin;
uniform vec3 viewRel;
in vec3 fragRel;
uniform vec3 globeCenter;
uniform mat4 globeRot; // world → Mars frame: turns the player's spot toward the viewer
uniform vec3 playerDir;
uniform float tilesPerFace;
uniform float time;
out vec4 finalColor;

float coverage(vec3 d) {
    vec3 a = abs(d);
    int f; vec3 N, U, V;
    if (a.x >= a.y && a.x >= a.z) {
        if (d.x > 0.0) { f = 0; N = vec3(1, 0, 0); U = vec3(0, 0, -1); V = vec3(0, 1, 0); }
        else           { f = 1; N = vec3(-1, 0, 0); U = vec3(0, 0, 1); V = vec3(0, 1, 0); }
    } else if (a.y >= a.z) {
        if (d.y > 0.0) { f = 2; N = vec3(0, 1, 0); U = vec3(1, 0, 0); V = vec3(0, 0, -1); }
        else           { f = 3; N = vec3(0, -1, 0); U = vec3(1, 0, 0); V = vec3(0, 0, 1); }
    } else {
        if (d.z > 0.0) { f = 4; N = vec3(0, 0, 1); U = vec3(1, 0, 0); V = vec3(0, 1, 0); }
        else           { f = 5; N = vec3(0, 0, -1); U = vec3(-1, 0, 0); V = vec3(0, 1, 0); }
    }
    float dn = dot(d, N);
    vec2 uv = vec2(atan(dot(d, U) / dn), atan(dot(d, V) / dn)) * (2.0 / 3.14159265) + 0.5;
    vec2 tile = min(floor(clamp(uv, 0.0, 1.0) * tilesPerFace), vec2(tilesPerFace - 1.0));
    return texelFetch(texture0, ivec2(tile) + ivec2(f - 3 * (f / 3), f / 3) * int(tilesPerFace), 0).r;
}

void main() {
    // Back to Mars' own frame.
    vec3 w = normalize(fragPos - globeCenter);
    vec3 d = normalize((globeRot * vec4(w, 0.0)).xyz);

    vec3 v = normalize(viewRel - fragRel);
    float rim = pow(1.0 - abs(dot(normalize(fragNormal), v)), 2.0);
    vec3 col = vec3(0.10, 0.45, 0.65) * (0.22 + rim * 0.9);

    float lat = asin(clamp(d.y, -1.0, 1.0)) / 0.2618;
    float lon = atan(d.z, d.x) / 0.2618;
    float grid = max(1.0 - smoothstep(0.0, 0.06, abs(fract(lat + 0.5) - 0.5)),
                     (1.0 - smoothstep(0.0, 0.06, abs(fract(lon + 0.5) - 0.5))) * sqrt(max(0.0, 1.0 - d.y * d.y)));
    col += vec3(0.20, 0.65, 0.90) * grid * 0.3;

    col = mix(col, vec3(0.75, 1.0, 1.0) * 1.3, clamp(coverage(d) * 3.0, 0.0, 1.0));
    col += vec3(1.0, 0.8, 0.3) * smoothstep(0.996, 0.999, d.y) * 2.0;
    float pulse = 0.6 + 0.4 * sin(time * 6.0);
    col += vec3(1.0, 0.35, 0.25) * smoothstep(0.997, 0.9993, dot(d, playerDir)) * 2.0 * pulse;
    col *= 0.85 + 0.15 * sin(fragPos.y * 90.0 + time * 3.0);
    finalColor = vec4(col, 1.0);
}
