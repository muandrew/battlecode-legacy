package lazy

import (
	"io"
	"github.com/labstack/echo"
	"html/template"
	"net/http"
	"github.com/muandrew/battlecode-ladder/auth"
	"github.com/muandrew/battlecode-ladder/models"
	"github.com/muandrew/battlecode-ladder/build"
	"github.com/muandrew/battlecode-ladder/data"
	"github.com/muandrew/battlecode-ladder/utils"
)

type LzSite struct {
	templates *template.Template
}

func NewInstance() *LzSite {
	return &LzSite{
		templates: template.Must(template.ParseGlob("lazy/views/*.html")),
	}
}

func (t *LzSite) Init(e *echo.Echo, a *auth.Auth, db data.Db, c *build.Ci) {
	e.Renderer = t
	g := e.Group("/lazy")
	g.Static("/static", "lazy/static")
	g.GET("/", getHello)
	g.GET("/login/", getLogin)
	r := g.Group("/loggedin")
	r.Use(a.AuthMiddleware)
	r.GET("/", wrapGetLoggedIn(db))
	r.POST("/upload/", wrapPostUpload(c))
	r.POST("/challenge/", wrapPostChallenge(db, c))

	if utils.IsDev() {
		d := g.Group("/dev")
		d.GET("/login/", wrapGetDevLogin(a))
		d.GET("/script/", getDevScript)
		d.POST("/script/", postDevScript)
	}
}

func (t *LzSite) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func getHello(c echo.Context) error {
	return c.Render(http.StatusOK, "root", nil)
}

func getLogin(c echo.Context) error {
	return c.Render(http.StatusOK, "login", nil)
}

func wrapGetDevLogin(a *auth.Auth) func(context echo.Context) error {
	return func(c echo.Context) error {
		a.GetUserWithApp(
			c,
			"dev",
			"#000000",
			func() *models.User {
				user, _ := models.CreateUser("Dev")
				return user
			},
		)
		return c.Redirect(http.StatusTemporaryRedirect, "/lazy/loggedin/")
	}
}

func getDevScript(c echo.Context) error {
	return c.Render(http.StatusOK, "dev_script", nil)
}

func postDevScript(c echo.Context) error {
	script := c.FormValue("script")
	utils.RunShell("sh", []string{"scripts/" + script});
	return c.Render(http.StatusOK, "dev_script", nil)
}

func wrapGetLoggedIn(db data.Db) func(context echo.Context) error {
	return func(c echo.Context) error {
		uuid := auth.GetUuid(c)
		bots, _ := db.GetBots(uuid, 0, 5)
		matches, length := db.GetMatches(uuid, 0, 5)
		model := map[string]interface{}{
			"name":           auth.GetName(c),
			"uuid":           uuid,
			"latest_bots":    bots,
			"latest_matches": matches,
			"length":         length,
		}
		return c.Render(http.StatusOK, "loggedin", model)
	}
}

func wrapPostUpload(ci *build.Ci) func(context echo.Context) error {
	return func(c echo.Context) error {
		uuid := auth.GetUuid(c)
		file, err := c.FormFile("file")
		if err != nil {
			return err
		}
		bot, err := models.CreateBot(
			models.NewCompetitor(models.CompetitorTypeUser, uuid),
			c.FormValue("package"),
			c.FormValue("note"),
			models.BotCompetitionBC17,
		)
		if err != nil {
			return err
		}

		err = ci.UploadBotSource(file, bot)
		if err != nil {
			return err
		}
		ci.SubmitJob(bot)
		return c.Render(http.StatusOK, "uploaded", auth.GetName(c))
	}
}

func wrapPostChallenge(db data.Db, ci *build.Ci) func(context echo.Context) error {
	return func(c echo.Context) error {
		botUuid := c.FormValue("botUuid")
		oppUuid := c.FormValue("oppUuid")

		ownBot := db.GetBot(botUuid)
		oppBot := db.GetBot(oppUuid)

		if ownBot != nil && oppBot != nil {
			ci.RunMatch(ownBot, oppBot)
			return c.Render(http.StatusOK, "challenged", nil)
		} else {
			return c.Render(http.StatusOK, "challenge_failed", nil)
		}
	}
}