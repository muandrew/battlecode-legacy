package auth

import (
	"github.com/labstack/echo"
	"github.com/muandrew/battlecode-ladder/models"
	jwt "github.com/dgrijalva/jwt-go"
	"time"
	"net/http"
	"github.com/muandrew/battlecode-ladder/db"
	"github.com/labstack/echo/middleware"
)

const jwtCookieName = "xbclauth"

type Auth struct {
	data db.Db
	jwtSecret []byte
	JwtConfig middleware.JWTConfig
}

func NewAuth(data db.Db, jwtSecret []byte) *Auth {
	config := middleware.DefaultJWTConfig
	config.SigningKey = jwtSecret
	config.TokenLookup = "cookie:" + jwtCookieName
	return &Auth{
		data:data,
		jwtSecret:jwtSecret,
		JwtConfig:config,
	}
}

func (auth Auth) GetUserWithApp(c echo.Context, app string, appUuid string, setupUser models.SetupNewUser) {
	user := auth.data.GetUserWithApp(app, appUuid, func() *models.User {
		user := models.CreateUserWithNewUuid()
		return setupUser(user)
	})
	auth.setJwtInCookie(c, user)
}

func (auth Auth) setJwtInCookie(c echo.Context, user *models.User) string {
	// Create token
	token := jwt.New(jwt.SigningMethodHS256)

	// Set claims
	claims := token.Claims.(jwt.MapClaims)
	claims["uuid"] = user.Uuid
	claims["name"] = user.Name
	claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

	// Generate encoded token and send it as response.
	t, err := token.SignedString(auth.jwtSecret)
	if err != nil {
		return ""
	}

	cookie := new(http.Cookie)
	cookie.Name = jwtCookieName
	cookie.Value = t
	cookie.Expires = time.Now().Add(time.Hour * 24)
	cookie.Path = "/"
	c.SetCookie(cookie)
	return t
}

