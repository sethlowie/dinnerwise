package meal

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	mealv1 "github.com/sethlowie/dinnerwise/internal/meal/v1"
	"github.com/sethlowie/dinnerwise/internal/meal/v1/mealv1connect"
)

// Service implements mealv1connect.MealServiceHandler by mapping the domain
// Repo onto proto. Sort/filter happen in the repo (server-side).
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) mealv1connect.MealServiceHandler {
	return &Service{repo: repo}
}

func (s *Service) ListMeals(
	ctx context.Context,
	req *connect.Request[mealv1.ListMealsRequest],
) (*connect.Response[mealv1.ListMealsResponse], error) {
	meals, err := s.repo.List(ctx, req.Msg.GetSort(), req.Msg.GetFavoritesOnly())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*mealv1.Meal, len(meals))
	for i := range meals {
		out[i] = toProtoMeal(meals[i])
	}
	return connect.NewResponse(&mealv1.ListMealsResponse{Meals: out}), nil
}

func (s *Service) GetMeal(
	ctx context.Context,
	req *connect.Request[mealv1.GetMealRequest],
) (*connect.Response[mealv1.GetMealResponse], error) {
	m, cooks, err := s.repo.GetByID(ctx, req.Msg.GetId())
	if errors.Is(err, ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	recent := make([]*mealv1.Cook, len(cooks))
	for i, c := range cooks {
		recent[i] = &mealv1.Cook{CookedOn: c.CookedOn, Note: c.Note}
	}
	return connect.NewResponse(&mealv1.GetMealResponse{
		Meal:        toProtoMeal(m),
		RecentCooks: recent,
	}), nil
}

func toProtoMeal(m Meal) *mealv1.Meal {
	return &mealv1.Meal{
		Id:          m.ID,
		Name:        m.Name,
		Cuisine:     m.Cuisine,
		Rating:      int32(m.Rating),
		TimesCooked: int32(m.TimesCooked),
		LastCooked:  m.LastCooked,
		RecipeId:    m.RecipeID,
	}
}
