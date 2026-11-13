// Cascadeur : des combos pour les sauts enchaînés, en JavaScript.
//
// game.vehicle()       le véhicule conduit, ou null : {id, speed, air, alt,
//                      airtime, fuel, boosting}
// game.push(avant, haut)  ajoute de la vitesse (m/s) au véhicule conduit
// évènements           jump (id, amount = vitesse), land (id, amount =
//                      secondes en l'air), boost (id)

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

game.debugLine(() => {
  const v = game.vehicle();
  const vol = v ? (v.air ? "en l'air " + v.airtime.toFixed(1) + " s" : "au sol") : "à pied";
  return "cascadeur : combo " + combo + ", record " + (game.get("record") || 0).toFixed(1) + " s, " + vol;
});
