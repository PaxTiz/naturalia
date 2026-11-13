// La serre : comportements en JavaScript (voir script.go pour l'API).
//
// game.on(évènement, fn)       tick, dust_collected, gem_collected, sell, buy,
//                              use, enter_building, leave_building, wake,
//                              place, drive, jump, land, boost
// game.randomEvent({id, every, when, run})
// lire  : credits(), tank(), capacity(), tool(), level(id), isDay(),
//         inside(), clock(), distanceToBase(), player()
// agir  : say(texte), giveCredits(n), burst(n, "#rrggbb"), beep(Hz),
//         dustDevil(), random()
//         spawn(entité), entities(entité), giveItem(objet, n), driving()
// véhicule : vehicle(), push(avant, haut) (voir examples/mods/cascadeur)
// garder dans la sauvegarde : get(clé), set(clé, valeur)
// mode debug (F3) : debugLine(() => "texte")

game.on("enter_building", e => {
  if (e.id === "serre:serre") {
    const visits = (game.get("visits") || 0) + 1;
    game.set("visits", visits);
    if (visits === 1) game.say("Bienvenue dans la serre : de l'oxygène et un peu de vert");
  }
});

// L'arbre, que l'on vient respirer.
game.on("use", e => {
  if (e.id !== "serre:arbre") return;
  game.say("Ça sent la chlorophylle. Le moral remonte.");
  game.burst(30, "#7cd36a");
});

// Largage de ravitaillement : loin de la base, de jour, de temps en temps.
game.randomEvent({
  id: "largage",
  every: 420,
  when: () => game.isDay() && game.distanceToBase() > 150,
  run: () => {
    const prime = Math.round(50 + game.credits() * 0.05);
    game.say("Largage de ravitaillement : la Terre vous envoie " + prime + " cr");
    game.giveCredits(prime);
    game.burst(60, "#ffd36a");
    game.beep(660);
  },
});

// Un tourbillon de plus quand on vend beaucoup d'un coup : Mars se venge.
game.on("sell", e => {
  if (e.amount > 5000 && game.random() < 0.3) game.dustDevil();
});

// Chaque cristal ramassé a une chance de laisser une pousse derrière lui.
game.on("gem_collected", () => {
  if (game.random() < 0.2) {
    game.spawn("pousse");
    game.say("Une pousse a germé là où dormait le cristal");
  }
});

game.debugLine(() => "serre : " + game.entities("pousse") + " pousses, " + (game.get("visits") || 0) + " visites");
