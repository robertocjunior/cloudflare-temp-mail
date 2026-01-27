package handlers

import (
	"context"
	"net/http"
	"strings"
	"tempmail/internal/database"
	"tempmail/internal/services"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// BLOQUEIO TOTAL: Se não houver usuários no DB, as APIs protegidas não funcionam.
		if !database.IsSetupDone() {
			http.Error(w, "Setup pendente. Crie um usuário primeiro.", http.StatusPreconditionFailed)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Autorização necessária", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		username, err := services.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Token inválido ou expirado", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "username", username)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}