// Cascadeur : des combos pour les sauts enchaînés, en JavaScript.
//
// game.vehicle()       le véhicule conduit, ou null : {id, speed, air, alt,
//                      airtime, fuel, boosting}
// game.push(avant, haut)  ajoute de la vitesse (m/s) au véhicule conduit
// game.impulse(entité, {x, y, z})  pareil pour une entité, axes du monde
// game.data(type)      les données "data" d'un type d'entité
// évènements           jump (id, amount = vitesse), land (id, amount =
//                      secondes en l'air), boost (id), impact (id, entity,
//                      amount = vitesse du choc), trigger_enter et
//                      trigger_leave (id, entity, by = "player" ou le
//                      véhicule)

let combo = 0;
let lastLand = -10;

// Retomber moins de 2 s après le saut précédent enchaîne le combo :
// le véhicule est relancé et le combo paie.
game.on("land", e => {
  if (e.amount < 0.8) return;
  combo = game.clock() - lastLand < 2 ? combo + 1 : 1;
  lastLand = game.clock();
  if (combo >= 2) {
    game.push(4 * combo, 0);
    game.say("Combo x" + combo + " !");
    game.giveCredits(100 * combo * combo);
    game.burst(20 * combo, "#c6ff3a");
  }
  const best = game.get("record") || 0;
  if (e.amount > best) {
    game.set("record", e.amount);
    if (best > 0) game.say("Record : " + e.amount.toFixed(1) + " s en l'air");
  }
});

// Sauter en plein boost envoie plus haut.
game.on("jump", () => {
  const v = game.vehicle();
  if (v && v.boosting) game.push(0, 3);
});

// Atterrir sur une cible, en vol : la prime de la cible, doublée en combo.
game.on("trigger_enter", e => {
  const v = game.vehicle();
  if (e.id !== "cascadeur:cible" || !v || !v.air) return;
  const prime = game.data("cible").prime * (combo >= 2 ? 2 : 1);
  game.say("En plein dans la cible !");
  game.giveCredits(prime);
  game.burst(40, "#c6ff3a");
});

// La moto envoie les caisses en l'air, bien plus haut que les autres.
game.on("impact", e => {
  if (e.id !== "core:crate" || game.driving() !== "cascadeur:moto") return;
  const l = Math.hypot(e.at.x, e.at.y, e.at.z); // vers le haut : loin du centre de Mars
  game.impulse(e.entity, {x: e.at.x / l * 5, y: e.at.y / l * 5, z: e.at.z / l * 5});
});

game.debugLine(() => {
  const v = game.vehicle();
  const vol = v ? (v.air ? "en l'air " + v.airtime.toFixed(1) + " s" : "au sol") : "à pied";
  return "cascadeur : combo " + combo + ", record " + (game.get("record") || 0).toFixed(1) + " s, " + vol;
});
