package meal

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	mealv1 "github.com/sethlowie/dinnerwise/internal/meal/v1"
)

func newSeededService(t *testing.T) *Service {
	t.Helper()
	database := newTestDB(t)
	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return &Service{repo: NewRepo(database)}
}

func TestServiceListMealsFavoritesByRating(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.ListMeals(context.Background(),
		connect.NewRequest(&mealv1.ListMealsRequest{Sort: "rating", FavoritesOnly: true}))
	if err != nil {
		t.Fatalf("ListMeals: %v", err)
	}
	if len(resp.Msg.Meals) != 7 { // design: 7 meals rated >= 4
		t.Fatalf("favorites = %d, want 7", len(resp.Msg.Meals))
	}
	for _, m := range resp.Msg.Meals {
		if m.GetRating() < 4 {
			t.Fatalf("non-favorite leaked: %+v", m)
		}
	}
	// top by rating is one of the 5-star meals
	if resp.Msg.Meals[0].GetRating() != 5 {
		t.Fatalf("top rating = %d, want 5", resp.Msg.Meals[0].GetRating())
	}
}

func TestServiceGetMeal(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.GetMeal(context.Background(),
		connect.NewRequest(&mealv1.GetMealRequest{Id: "salmon"}))
	if err != nil {
		t.Fatalf("GetMeal: %v", err)
	}
	if resp.Msg.Meal.GetTimesCooked() != 7 {
		t.Fatalf("times_cooked = %d, want 7", resp.Msg.Meal.GetTimesCooked())
	}
	if len(resp.Msg.RecentCooks) == 0 {
		t.Fatal("expected recent cooks")
	}
}

func TestServiceGetMealNotFound(t *testing.T) {
	svc := newSeededService(t)
	_, err := svc.GetMeal(context.Background(),
		connect.NewRequest(&mealv1.GetMealRequest{Id: "nope"}))
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Fatalf("code = %v, want NotFound", got)
	}
}
