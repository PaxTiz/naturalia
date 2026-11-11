#version 330
// fxaa.frag smooths the jagged edges of the graded image (FXAA 3.11, quality
// preset 12): it finds where the contrast is, which way the edge runs,
// walks along it to both ends, and blends across it by how far the pixel
// is from the nearer end. Then a fine film grain.
in vec2 fragTexCoord;
uniform sampler2D texture0; // rgb: image, a: luma
uniform float time;
out vec4 finalColor;

float L(vec2 p) { return texture(texture0, p).a; }

void main() {
    vec2 uv = fragTexCoord;
    vec2 rcp = 1.0 / vec2(textureSize(texture0, 0));
    vec4 rgbyM = texture(texture0, uv);
    float lumaM = rgbyM.a;
    float lumaS = L(uv + vec2(0, 1) * rcp), lumaE = L(uv + vec2(1, 0) * rcp);
    float lumaN = L(uv + vec2(0, -1) * rcp), lumaW = L(uv + vec2(-1, 0) * rcp);
    float rangeMax = max(max(max(lumaS, lumaE), max(lumaN, lumaW)), lumaM);
    float rangeMin = min(min(min(lumaS, lumaE), min(lumaN, lumaW)), lumaM);
    float range = rangeMax - rangeMin;
    vec3 col = rgbyM.rgb;
    if (range >= max(0.0312, rangeMax * 0.125)) {
        float lumaNW = L(uv + vec2(-1, -1) * rcp), lumaSE = L(uv + vec2(1, 1) * rcp);
        float lumaNE = L(uv + vec2(1, -1) * rcp), lumaSW = L(uv + vec2(-1, 1) * rcp);
        float lumaNS = lumaN + lumaS, lumaWE = lumaW + lumaE;
        float subpix = (2.0 * (lumaNS + lumaWE) + lumaNW + lumaSE + lumaNE + lumaSW) / 12.0;
        subpix = clamp(abs(subpix - lumaM) / range, 0.0, 1.0);
        subpix = smoothstep(0.0, 1.0, subpix);
        subpix = subpix * subpix * 0.75;
        float edgeH = abs(lumaNW + lumaNE - 2.0 * lumaN) + 2.0 * abs(lumaWE - 2.0 * lumaM) + abs(lumaSW + lumaSE - 2.0 * lumaS);
        float edgeV = abs(lumaNW + lumaSW - 2.0 * lumaW) + 2.0 * abs(lumaNS - 2.0 * lumaM) + abs(lumaNE + lumaSE - 2.0 * lumaE);
        bool horz = edgeH >= edgeV;
        float step = horz ? rcp.y : rcp.x;
        float luma1 = horz ? lumaN : lumaW, luma2 = horz ? lumaS : lumaE;
        float grad1 = abs(luma1 - lumaM), grad2 = abs(luma2 - lumaM);
        bool steep1 = grad1 >= grad2;
        float gradScaled = 0.25 * max(grad1, grad2);
        float lumaLocal;
        if (steep1) { step = -step; lumaLocal = 0.5 * (luma1 + lumaM); }
        else lumaLocal = 0.5 * (luma2 + lumaM);
        vec2 cur = uv;
        if (horz) cur.y += step * 0.5; else cur.x += step * 0.5;
        vec2 off = horz ? vec2(rcp.x, 0.0) : vec2(0.0, rcp.y);
        const float steps[10] = float[10](1.0, 1.5, 2.0, 2.0, 2.0, 2.0, 4.0, 4.0, 8.0, 8.0);
        vec2 p1 = cur - off, p2 = cur + off;
        float e1 = L(p1) - lumaLocal, e2 = L(p2) - lumaLocal;
        bool done1 = abs(e1) >= gradScaled, done2 = abs(e2) >= gradScaled;
        for (int i = 1; i < 10 && !(done1 && done2); i++) {
            if (!done1) { p1 -= off * steps[i]; e1 = L(p1) - lumaLocal; done1 = abs(e1) >= gradScaled; }
            if (!done2) { p2 += off * steps[i]; e2 = L(p2) - lumaLocal; done2 = abs(e2) >= gradScaled; }
        }
        float d1 = horz ? uv.x - p1.x : uv.y - p1.y;
        float d2 = horz ? p2.x - uv.x : p2.y - uv.y;
        bool near1 = d1 < d2;
        float dist = min(d1, d2);
        float span = d1 + d2;
        bool centerSmaller = lumaM - lumaLocal < 0.0;
        bool good = ((near1 ? e1 : e2) < 0.0) != centerSmaller;
        float edgeOff = good ? 0.5 - dist / span : 0.0;
        float o = max(edgeOff, subpix);
        vec2 fuv = uv;
        if (horz) fuv.y += o * step; else fuv.x += o * step;
        col = texture(texture0, fuv).rgb;
    }
    float n = fract(sin(dot(uv * 1000.0 + time, vec2(12.9898, 78.233))) * 43758.5453);
    col += (n - 0.5) * 0.02;
    finalColor = vec4(col, 1.0);
}
