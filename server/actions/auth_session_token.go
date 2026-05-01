package actions

import (
	"time"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

func newAuthSessionToken(secret []byte, tokenLifeTime int, jwtTokenIssuer string, existingUser map[string]interface{}, issuedAt time.Time, extraClaims map[string]interface{}) (string, error) {
	u, _ := uuid.NewV7()
	claims := jwt.MapClaims{
		"email": existingUser["email"],
		"sub":   daptinid.InterfaceToDIR(existingUser["reference_id"]).String(),
		"name":  existingUser["name"],
		"nbf":   issuedAt.Unix(),
		"exp":   issuedAt.Add(time.Duration(tokenLifeTime) * time.Hour).Unix(),
		"iss":   jwtTokenIssuer,
		"iat":   issuedAt.Unix(),
		"jti":   u.String(),
		auth.AuthVersionClaim: auth.AuthVersionOrDefault(
			existingUser[auth.AuthVersionColumn],
		),
	}
	for key, value := range extraClaims {
		claims[key] = value
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}
