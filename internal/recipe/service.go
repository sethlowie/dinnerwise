package recipe

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	recipev1 "github.com/sethlowie/dinnerwise/internal/recipe/v1"
	"github.com/sethlowie/dinnerwise/internal/recipe/v1/recipev1connect"
)

// Service implements recipev1connect.RecipeServiceHandler by mapping the
// domain Repo onto the proto API.
type Service struct {
	repo *Repo
}

// NewService returns a RecipeServiceHandler backed by repo.
func NewService(repo *Repo) recipev1connect.RecipeServiceHandler {
	return &Service{repo: repo}
}

func (s *Service) ListRecipes(
	ctx context.Context,
	req *connect.Request[recipev1.ListRecipesRequest],
) (*connect.Response[recipev1.ListRecipesResponse], error) {
	recs, err := s.repo.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*recipev1.Recipe, len(recs))
	for i := range recs {
		out[i] = toProtoRecipe(recs[i])
	}
	return connect.NewResponse(&recipev1.ListRecipesResponse{Recipes: out}), nil
}

func (s *Service) GetRecipe(
	ctx context.Context,
	req *connect.Request[recipev1.GetRecipeRequest],
) (*connect.Response[recipev1.GetRecipeResponse], error) {
	rec, err := s.repo.GetByID(ctx, req.Msg.GetId())
	if errors.Is(err, ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&recipev1.GetRecipeResponse{Recipe: toProtoRecipe(rec)}), nil
}

// toProtoRecipe maps a domain Recipe to its proto representation.
func toProtoRecipe(r Recipe) *recipev1.Recipe {
	ingredients := make([]*recipev1.RecipeIngredient, len(r.Ingredients))
	for i, ing := range r.Ingredients {
		ingredients[i] = &recipev1.RecipeIngredient{
			IngredientId: ing.IngredientID,
			Name:         ing.Name,
			Quantity:     ing.Quantity,
			Unit:         ing.Unit,
		}
	}
	return &recipev1.Recipe{
		Id:           r.ID,
		Name:         r.Name,
		Cuisine:      r.Cuisine,
		Difficulty:   r.Difficulty,
		Servings:     int32(r.Servings),
		TotalMinutes: int32(r.TotalMinutes),
		Steps:        r.Steps,
		Ingredients:  ingredients,
		InPantry:     r.InPantry,
	}
}
