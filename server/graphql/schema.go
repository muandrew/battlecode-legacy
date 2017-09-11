package graphql

import (
	"fmt"
	"net/http"
	"github.com/graphql-go/graphql"
	"github.com/labstack/echo"
	"github.com/muandrew/battlecode-ladder/models"
	"github.com/muandrew/battlecode-ladder/data"
)

type Request struct {
	Query string `json:"query"`
}

func rootQuery(db data.Db) *graphql.Object {
	bcMapType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "BCMap",
		Description: "Map, say Map!",
		Fields: graphql.Fields{
			"uuid": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Name:        "Uuid of the map",
				Description: "A map's uuid.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if m, ok := p.Source.(*models.BcMap); ok {
						return m.Uuid, nil
					}
					return nil, nil
				},
			},
			"name": &graphql.Field{
				Type:        graphql.String,
				Name:        "Name of the map",
				Description: "A map.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if m, ok := p.Source.(*models.BcMap); ok {
						return m.Name, nil
					}
					return nil, nil
				},
			},
			"description": &graphql.Field{
				Type:        graphql.String,
				Name:        "Some nice description",
				Description: "A map's description.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if m, ok := p.Source.(*models.BcMap); ok {
						return m.Description, nil
					}
					return nil, nil
				},
			},
		},
	})

	matchType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "Match",
		Description: "A match between bots",
		Fields: graphql.Fields{
			"uuid": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "https://segment.com/blog/a-brief-history-of-the-uuid/",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if m, ok := p.Source.(*data.Match); ok {
						return m.Uuid, nil
					}
					return nil, nil
				},
			},
			"mapUuid": &graphql.Field{
				Type:        graphql.String,
				Description: "The user's display name.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if m, ok := p.Source.(*data.Match); ok {
						return m.MapUuid, nil
					}
					return nil, nil
				},
			},
			"map": &graphql.Field{
				Type:        bcMapType,
				Description: "The map the game was played on",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if m, ok := p.Source.(*data.Match); ok {
						if m.MapUuid == "" {
							return nil, nil
						} else {
							return db.GetBcMap(m.MapUuid), nil
						}
					}
					return nil, nil
				},
			},
		},
	})

	return graphql.NewObject(graphql.ObjectConfig{
		Name:        "Query",
		Description: "Root query",
		Fields: graphql.Fields{
			"user": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name:        "User",
					Description: "A user of the battlecode-ladder system.",
					Fields: graphql.Fields{
						"uuid": &graphql.Field{
							Type:        graphql.NewNonNull(graphql.String),
							Description: "https://segment.com/blog/a-brief-history-of-the-uuid/",
							Resolve: func(p graphql.ResolveParams) (interface{}, error) {
								if user, ok := p.Source.(*models.User); ok {
									return user.Uuid, nil
								}
								return nil, nil
							},
						},
						"name": &graphql.Field{
							Type:        graphql.NewNonNull(graphql.String),
							Description: "The user's display name.",
							Resolve: func(p graphql.ResolveParams) (interface{}, error) {
								if user, ok := p.Source.(*models.User); ok {
									return user.Name, nil
								}
								return nil, nil
							},
						},
						"map": &graphql.Field{
							Type:        bcMapType,
							Description: "The map the game was played on",

							Resolve: func(p graphql.ResolveParams) (interface{}, error) {
								if m, ok := p.Source.(*models.Match); ok {
									return db.GetBcMap(m.MapUuid), nil
								}
								return nil, nil

							},
						},
					},
				}),
				Description: "gets a user",
				Args: graphql.FieldConfigArgument{
					"uuid": &graphql.ArgumentConfig{
						Type:        graphql.String,
						Description: "A user's uuid.",
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return db.GetUser(p.Args["uuid"].(string)), nil
				},
			},
			"match": &graphql.Field{
				Type:        matchType,
				Name:        "Match",
				Description: "Getting a match",
				Args: graphql.FieldConfigArgument{
					"uuid": &graphql.ArgumentConfig{
						Type:        graphql.String,
						Description: "A match's uuid.",
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return db.GetMatch(p.Args["uuid"].(string))
				},
			},
			"map": &graphql.Field{
				Type:        bcMapType,
				Name:        "Map",
				Description: "Getting a map.",
				Args: graphql.FieldConfigArgument{
					"uuid": &graphql.ArgumentConfig{
						Description: "A map's uuid.",
						Type:        graphql.String,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return db.GetBcMap(p.Args["uuid"].(string)), nil
				},
			},
		},
	})
}

func schema(db data.Db) (graphql.Schema, error) {
	return graphql.NewSchema(graphql.SchemaConfig{
		Query: rootQuery(db),
	})
}

func executeQuery(query string, schema graphql.Schema) *graphql.Result {
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) > 0 {
		fmt.Printf("wrong result, unexpected errors: %v", result.Errors)
	}
	return result
}

func Init(db data.Db, e *echo.Echo) error {
	schema, err := schema(db)
	if err != nil {
		return err
	}
	e.GET("graphql/", func(context echo.Context) error {
		result := executeQuery(context.QueryParam("query"), schema)
		return context.JSON(http.StatusOK, result)
	})
	e.POST("graphql/", func(context echo.Context) error {
		request := &Request{}
		err := context.Bind(request)
		if err != nil {
			return err
		}
		result := executeQuery(request.Query, schema)
		return context.JSON(http.StatusOK, result)
	})
	return nil
}
