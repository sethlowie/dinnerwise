CREATE TABLE IF NOT EXISTS meal (
  id        TEXT PRIMARY KEY,
  name      TEXT NOT NULL,
  cuisine   TEXT NOT NULL DEFAULT '',
  rating    INTEGER NOT NULL DEFAULT 0,
  recipe_id TEXT REFERENCES recipe(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS meal_cook (
  id        INTEGER PRIMARY KEY,
  meal_id   TEXT NOT NULL REFERENCES meal(id) ON DELETE CASCADE,
  cooked_on TEXT NOT NULL,
  note      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_meal_cook_meal ON meal_cook(meal_id);
