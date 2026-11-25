// The building grid's lines, painted on the ground and on what is built
// around the ghost, as they lie: every gridStep along its axes. A line
// shows on the faces it runs along.
uniform vec3 gridRel;  // a point of its lines, about origin
uniform vec3 gridX, gridUp, gridZ;
uniform vec2 gridSize; // x: between lines, y: how far they show (0 hides them)
vec3 buildGrid(vec3 col, vec3 n) {
    if (gridSize.y <= 0.0) return col;
    vec3 r = fragRel - gridRel;
    vec3 p = vec3(dot(r, gridX), dot(r, gridUp), dot(r, gridZ));
    float fade = 1.0 - smoothstep(0.3, 1.0, length(p) / gridSize.y);
    if (fade <= 0.0) return col;
    vec3 q = p / gridSize.x;
    vec3 off = abs(q - floor(q + 0.5)) * gridSize.x;
    vec3 px = fwidth(p);
    vec3 line = 1.0 - smoothstep(vec3(0.008), vec3(0.008) + px * 1.2 + 0.004, off);
    line *= 1.0 - smoothstep(0.15, 0.4, px / gridSize.x); // too far to tell apart
    vec3 face = abs(vec3(dot(n, gridX), dot(n, gridUp), dot(n, gridZ)));
    line *= 1.0 - smoothstep(0.6, 0.8, face);
    float a = max(line.x, max(line.y, line.z));
    return mix(col, vec3(0.45, 1.0, 0.95), a * fade * 0.6);
}
