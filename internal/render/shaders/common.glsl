// common.glsl holds the noise, sky, lighting and haze shared by all
// fragment shaders. The sun is fixed in space: walking around Mars brings
// sunsets and nights, so the sky and the fill light follow the local up.
uniform vec3 viewPos;
uniform vec3 sunDir;
// Positions from Mars' center are about 3 400 m long: in 32-bit floats they
// are only good to a quarter millimeter, which shakes everything computed
// from small differences (slopes, bumps, facets, depth) as the camera moves.
// So the vertex shaders also give fragRel, the position from origin (a
// point near the camera), and the fine work uses it with viewRel.
uniform vec3 origin;
uniform vec3 viewRel; // viewPos - origin, computed in double precision
// hazeThin shortens the haze's distances when the eye is high above the
// ground (creative flight): the air is thinner up there. 1 on foot.
uniform float hazeThin;
in vec3 fragRel;

const vec3 phobosDir = vec3(0.451, 0.551, -0.702);
const vec3 deimosDir = vec3(-0.204, 0.865, 0.458);

float hash13(vec3 p) {
    p = fract(p * 0.1031);
    p += dot(p, p.zyx + 31.32);
    return fract((p.x + p.y) * p.z);
}

float vnoise3(vec3 p) {
    vec3 i = floor(p), f = fract(p);
    vec3 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(mix(hash13(i), hash13(i + vec3(1, 0, 0)), u.x),
                   mix(hash13(i + vec3(0, 1, 0)), hash13(i + vec3(1, 1, 0)), u.x), u.y),
               mix(mix(hash13(i + vec3(0, 0, 1)), hash13(i + vec3(1, 0, 1)), u.x),
                   mix(hash13(i + vec3(0, 1, 1)), hash13(i + vec3(1, 1, 1)), u.x), u.y), u.z);
}

float fbm3(vec3 p) {
    float s = 0.0, a = 0.5;
    for (int i = 0; i < 5; i++) { s += a * vnoise3(p); p = p * 2.03 + 17.0; a *= 0.5; }
    return s;
}

// daylight: 1 when the sun is well above the local horizon, 0 at night.
float daylight(vec3 up) { return smoothstep(-0.12, 0.18, dot(sunDir, up)); }

// skyColor is the color of the sky seen along d: three flat bands softly
// blended (golden peach horizon, dusty rose, deep violet overhead), a small bluish
// halo around the sun, a blue glow at sunset. At night the horizon keeps a
// faint violet glow so the relief stands out as a silhouette.
vec3 skyColor(vec3 d, vec3 up) {
    float h = dot(d, up);
    float s = max(dot(d, sunDir), 0.0);
    float day = daylight(up);
    vec3 col = mix(vec3(1.00, 0.70, 0.46), vec3(0.82, 0.42, 0.38), smoothstep(-0.02, 0.18, h));
    col = mix(col, vec3(0.30, 0.17, 0.34), smoothstep(0.14, 0.8, h));
    col = mix(col, vec3(0.78, 0.86, 1.05), pow(s, 48.0) * 0.5);
    col += vec3(1.0, 0.72, 0.48) * pow(s, 10.0) * 0.07;
    vec3 night = mix(vec3(0.075, 0.07, 0.14), vec3(0.010, 0.012, 0.035), smoothstep(-0.05, 0.4, h));
    col = mix(night, col, day);
    float dusk = smoothstep(-0.35, -0.05, dot(sunDir, up)) * (1.0 - day * 0.7);
    col += vec3(0.25, 0.45, 0.95) * pow(s, 6.0) * dusk * 1.4;
    return col;
}

// Light, painted rather than measured, as in Firewatch: a face is in the
// light or in the shade, with a soft edge between and a gentle gradient on
// the lit side to keep forms round. The shade is a color of its own, cool
// and rich (plum by day, blue at dusk, deep blue at night), warmed from
// below by the lit ground. Highlights are rare soft spots; metals take a
// flat tint of what they mirror; silhouettes catch a little sky.

// lightRamp turns the cosine to a light into the painted ramp.
float lightRamp(float nl) {
    return mix(smoothstep(-0.04, 0.2, nl), clamp(nl, 0.0, 1.0), 0.35);
}

// glint is a painted highlight: a soft-edged spot, larger on matte
// surfaces, none at all when spec is 0.
float glint(vec3 n, vec3 v, vec3 l, float spec) {
    float nh = max(dot(n, normalize(l + v)), 0.0);
    float size = mix(0.93, 0.992, spec);
    return smoothstep(size, mix(size, 1.0, 0.4), nh) * spec;
}

const vec3 sunCol    = vec3(1.42, 1.05, 0.76); // sunlight through the dust: warm
const vec3 groundAlb = vec3(0.50, 0.27, 0.15); // what the ground bounces
const vec3 moonCol   = vec3(0.10, 0.13, 0.30); // Phobos and Deimos, by night

// shadeColor is the light in the shade: the sky's hue made deeper and
// richer.
vec3 shadeColor(vec3 up) {
    float s = dot(sunDir, up);
    float dusk = smoothstep(-0.35, -0.05, s) * (1.0 - smoothstep(0.05, 0.35, s));
    vec3 c = mix(vec3(0.05, 0.06, 0.15), vec3(0.30, 0.29, 0.52), daylight(up));
    return mix(c, vec3(0.24, 0.28, 0.62), dusk * 0.7);
}

// skyLight is what the sky alone gives a face turned toward n (the dust
// devils use it).
vec3 skyLight(vec3 n, vec3 up) {
    return shadeColor(up) * mix(0.6, 1.0, smoothstep(-0.7, 0.7, dot(n, up)));
}

// shadeM lights an outdoor surface: the sun (dimmed by shadow), the moon's
// cold key at night, the shade (dimmed by ao). spec is how glossy the
// surface is (0..1), metal how metallic.
vec3 shadeM(vec3 albedo, vec3 n, vec3 v, vec3 up, float shadow, float ao, float spec, float metal) {
    float day = daylight(up);
    float u = dot(n, up);
    float sun = lightRamp(dot(n, sunDir)) * shadow * day;
    vec3 mk = normalize(up + 0.9 * normalize(phobosDir - up * dot(phobosDir, up) + vec3(1e-4)));
    float moon = lightRamp(dot(n, mk)) * (1.0 - day);
    vec3 shade = shadeColor(up);
    vec3 bounce = groundAlb * sunCol * max(dot(sunDir, up), 0.0) * 0.5 + shade * 0.45;
    vec3 amb = mix(bounce, shade, smoothstep(-0.9, 0.3, u)) * ao;
    vec3 col = albedo * (1.0 - 0.45 * metal) * (sunCol * sun + moonCol * moon + amb);
    col += sunCol * glint(n, v, sunDir, max(spec, metal)) * sun * mix(vec3(0.45), albedo * 1.4, metal);
    vec3 r = reflect(-v, n);
    vec3 env = mix(bounce, skyColor(r, up), smoothstep(-0.15, 0.05, dot(r, up)));
    col += env * albedo * metal * 0.5 * ao;
    // The sky behind a silhouette rims its edge.
    vec3 back = -v - up * dot(-v, up);
    vec3 rimSky = skyColor(normalize(back + up * (0.12 * length(back) + 1e-3)), up);
    float rim = pow(1.0 - max(dot(n, v), 0.0), 4.0) * (1.0 - smoothstep(0.2, 0.7, u));
    return col + rimSky * rim * 0.3 * ao;
}

vec3 shade(vec3 albedo, vec3 n, vec3 v, vec3 up, float shadow, float ao, float spec) {
    return shadeM(albedo, n, v, up, shadow, ao, spec, 0.0);
}

// baseAO is the ambient occlusion of the base, worked out from its known
// shapes rather than from the screen: it holds still as the camera moves.
// Each plane nearby (floors and ground, walls, corridor ceilings) hides
// part of the sky or of the bounce from a surface, the more so the closer
// it is and the more the surface faces it: corners darken, the foot of
// the walls and of the objects standing on the ground too.
uniform vec3 baseRel;          // the base's origin, from origin
uniform vec4 baseRooms[8];     // local center x and z, radius, wall height
uniform vec4 baseCorrs[8];     // local ends: from x, z, to x, z
uniform vec4 baseCorrSize[8];  // width, height
uniform float roomCount, corrCount;
uniform vec4 baseBlobs[48];    // furniture and yard objects: local x, z, radius
uniform float blobCount;

float occ(float d, float r, vec3 n, vec3 pn) {
    d = max(d, 0.0);
    return (1.0 - d / sqrt(d * d + r * r)) * (0.5 - 0.5 * dot(n, pn));
}

float baseAO(vec3 n) {
    vec3 p = fragRel - baseRel;
    if (dot(p.xz, p.xz) > 1600.0) return 1.0;
    float ao = 1.0 - 0.7 * occ(p.y - 0.02, 0.8, n, vec3(0, 1, 0));
    bool inCorr = false;
    for (int i = 0; i < 8; i++) {
        if (float(i) >= corrCount) break;
        vec2 a = baseCorrs[i].xy;
        vec2 ax = baseCorrs[i].zw - a;
        float len = length(ax);
        ax /= len;
        vec2 side = vec2(-ax.y, ax.x);
        float t = dot(p.xz - a, ax), across = dot(p.xz - a, side);
        float hw = baseCorrSize[i].x * 0.5, h = baseCorrSize[i].y;
        if (t < -0.2 || t > len + 0.2) continue;
        vec3 sn = vec3(side.x, 0.0, side.y) * sign(across);
        if (abs(across) < hw + 0.05 && p.y < h + 0.1) {
            inCorr = true;
            ao *= 1.0 - 0.7 * occ(hw - abs(across), 0.8, n, -sn);
            ao *= 1.0 - 0.7 * occ(h - p.y, 0.8, n, vec3(0, -1, 0));
        } else if (abs(across) > hw + 0.1) {
            ao *= 1.0 - 0.65 * occ(abs(across) - hw - 0.17, 1.4, n, sn) * (1.0 - smoothstep(h, h + 0.8, p.y));
        }
    }
    // Under and around what stands on the floor, the floor darkens.
    if (p.y < 0.4 && n.y > 0.5) {
        for (int i = 0; i < 48; i++) {
            if (float(i) >= blobCount) break;
            float d = length(p.xz - baseBlobs[i].xy) / baseBlobs[i].z;
            ao *= 1.0 - 0.4 * (1.0 - smoothstep(0.5, 1.35, d));
        }
    }
    for (int i = 0; i < 8; i++) {
        if (float(i) >= roomCount) break;
        vec2 q = p.xz - baseRooms[i].xy;
        float r = max(length(q), 1e-3);
        vec3 rn = vec3(q.x, 0.0, q.y) / r;
        float R = baseRooms[i].z, H = baseRooms[i].w;
        if (r < R) ao *= 1.0 - 0.7 * occ(R - 0.1 - r, 0.8, n, -rn);
        else if (!inCorr) ao *= 1.0 - 0.65 * occ(r - R - 0.14, 1.4, n, rn) * (1.0 - smoothstep(H, H + 1.5, p.y));
    }
    return ao;
}

// finish fades distant surfaces into the dusty air: first a violet veil,
// then the sky's own color along the view, so relief stacks up in flat
// layers. The result is HDR: the exposure and the tone curve come later,
// on the whole image (post.go).
vec3 finish(vec3 col, float dist) {
    dist *= hazeThin;
    vec3 d = normalize(fragRel - viewRel);
    vec3 haze = skyColor(d, normalize(viewPos));
    vec3 veil = haze * vec3(0.62, 0.52, 0.72);
    col = mix(col, veil, (1.0 - exp(-dist / 1300.0)) * 0.42);
    return mix(col, haze, 1.0 - exp(-pow(dist / 3300.0, 1.6)));
}
