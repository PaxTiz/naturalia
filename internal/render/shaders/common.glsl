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
// blended (peach horizon, dusty rose, deep wine overhead), a small bluish
// halo around the sun, a blue glow at sunset. At night the horizon keeps a
// faint violet glow so the relief stands out as a silhouette.
vec3 skyColor(vec3 d, vec3 up) {
    float h = dot(d, up);
    float s = max(dot(d, sunDir), 0.0);
    float day = daylight(up);
    vec3 col = mix(vec3(1.00, 0.58, 0.36), vec3(0.78, 0.36, 0.28), smoothstep(-0.02, 0.16, h));
    col = mix(col, vec3(0.34, 0.13, 0.16), smoothstep(0.12, 0.75, h));
    col = mix(col, vec3(0.78, 0.86, 1.05), pow(s, 48.0) * 0.5);
    col += vec3(1.0, 0.72, 0.48) * pow(s, 10.0) * 0.07;
    vec3 night = mix(vec3(0.075, 0.07, 0.14), vec3(0.010, 0.012, 0.035), smoothstep(-0.05, 0.4, h));
    col = mix(night, col, day);
    float dusk = smoothstep(-0.35, -0.05, dot(sunDir, up)) * (1.0 - day * 0.7);
    col += vec3(0.25, 0.45, 0.95) * pow(s, 6.0) * dusk * 1.4;
    return col;
}

// Light, one model for everything, slightly physical: diffuse that keeps
// the energy the gloss reflects, GGX highlights with Fresnel, metals that
// tint their reflections. The ambient comes from the sky itself and the
// ground it lights, so shade always matches the sky above it. The colors
// stay the style's: a warm sun, a dusty rose sky, cool deep shade.
const float PI = 3.14159265;

vec3 fresnel(vec3 f0, float c) { return f0 + (1.0 - f0) * pow(1.0 - c, 5.0); }

// direct is one light of color c (irradiance on a face turned to it)
// coming from l. rough: 0 mirror, 1 chalk.
vec3 direct(vec3 albedo, vec3 f0, float rough, vec3 n, vec3 v, vec3 l, vec3 c) {
    float nl = dot(n, l);
    // Dusty air and soft paint wrap the light a little past the terminator.
    float diff = max(nl + 0.12, 0.0) / 1.12;
    vec3 h = normalize(l + v);
    float nh = max(dot(n, h), 0.0), nv = max(dot(n, v), 1e-3), cl = max(nl, 0.0);
    float a = max(rough * rough, 0.02), a2 = a * a;
    float d = nh * nh * (a2 - 1.0) + 1.0;
    float k = a * 0.5;
    vec3 F = fresnel(f0, max(dot(v, h), 0.0));
    vec3 spec = F * a2 / (d * d) / ((cl * (1.0 - k) + k) * (nv * (1.0 - k) + k)) * 0.25 * cl;
    return (albedo * (1.0 - F) * diff + spec) * c;
}

// ambient adds light from all around: irradiance sky from above and
// ground from below, then what the surroundings mirror (env) by Fresnel.
vec3 ambient(vec3 albedo, vec3 f0, float rough, vec3 n, vec3 v, vec3 up, vec3 sky, vec3 ground, vec3 env, float ao) {
    float u = dot(n, up);
    vec3 irr = mix(ground, sky, smoothstep(-0.9, 0.9, u));
    float nv = max(dot(n, v), 0.0);
    vec3 F = f0 + (max(vec3(1.0 - rough), f0) - f0) * pow(1.0 - nv, 5.0);
    return (albedo * (1.0 - F) * irr + env * F * (1.0 - rough * 0.7)) * ao;
}

const vec3 sunCol    = vec3(1.42, 1.05, 0.76); // sunlight through the dust: warm
const vec3 groundAlb = vec3(0.50, 0.27, 0.15); // what the ground bounces
const vec3 moonCol   = vec3(0.10, 0.13, 0.30); // Phobos and Deimos, by night

// skyLight is the irradiance from the sky on a face turned toward n:
// the sky's own color seen above it, a little cooled in the shade.
vec3 skyLight(vec3 n, vec3 up) {
    vec3 t = n - up * dot(n, up);
    vec3 hd = dot(t, t) > 1e-4 ? normalize(t) : normalize(cross(up, vec3(0.3, 0.9, 0.1)));
    vec3 high = skyColor(normalize(up + hd * 0.5), up);
    vec3 low = skyColor(normalize(hd + up * 0.25), up);
    // By night the dark sky alone would leave the relief black: starlight
    // and the moons' glow keep it a readable deep blue.
    vec3 night = vec3(0.025, 0.03, 0.065) * (1.0 - daylight(up));
    return mix(low, high, 0.5) * vec3(0.78, 0.82, 1.0) * 0.62 + night;
}

// shadeM lights an outdoor surface: the sun (dimmed by shadow), the moon's
// cold key at night, the sky and the lit ground around (dimmed by ao).
// spec is how glossy the surface is (0..1), metal how metallic.
vec3 shadeM(vec3 albedo, vec3 n, vec3 v, vec3 up, float shadow, float ao, float spec, float metal) {
    float day = daylight(up);
    float rough = mix(0.9, 0.22, spec);
    vec3 f0 = mix(vec3(0.04), albedo, metal);
    vec3 diff = albedo * (1.0 - metal);
    vec3 col = direct(diff, f0, rough, n, v, sunDir, sunCol * day * shadow);
    vec3 mk = normalize(up + 0.9 * normalize(phobosDir - up * dot(phobosDir, up) + vec3(1e-4)));
    col += direct(diff, f0, max(rough, 0.35), n, v, mk, moonCol * (1.0 - day));
    vec3 sky = skyLight(n, up);
    vec3 ground = groundAlb * (sunCol * max(dot(sunDir, up), 0.0) * 0.3 + sky * 0.8);
    vec3 env = skyColor(reflect(-v, n), up);
    env = mix(ground * 0.6, env, smoothstep(-0.15, 0.05, dot(reflect(-v, n), up)));
    return col + ambient(diff, f0, rough, n, v, up, sky, ground, env, ao);
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
