package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/repository"
	firebasepkg "github.com/smatch/badminton-backend/platform/firebase"
)

type contextKey string

const (
	CtxKeyUser     contextKey = "user"
	CtxKeyFirebase contextKey = "firebaseToken"
)

type AuthMiddleware struct {
	firebase    *firebasepkg.Client
	userRepo    *repository.UserRepository
	adminSecret string
}

func NewAuthMiddleware(fb *firebasepkg.Client, ur *repository.UserRepository, adminSecret string) *AuthMiddleware {
	return &AuthMiddleware{firebase: fb, userRepo: ur, adminSecret: adminSecret}
}

// extractBearerToken parses "Bearer <token>" from Authorization header.
func extractBearerToken(h string) string {
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// sendError writes a JSON error response (matching response.go format).
func sendError(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"success":false,"error":{"message":"` + message + `","code":"` + code + `"}}`)) //nolint:errcheck
}

// UserFromContext extracts the authenticated user from context.
func UserFromContext(ctx context.Context) *domain.User {
	u, _ := ctx.Value(CtxKeyUser).(*domain.User)
	return u
}

// RequireAuth verifies the Firebase ID token and loads the user from DB.
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			sendError(w, "Authorization header required", "MISSING_TOKEN", 401)
			return
		}

		decoded, err := m.firebase.VerifyIDToken(r.Context(), token)
		if err != nil {
			sendError(w, "Invalid or expired token", "INVALID_TOKEN", 401)
			return
		}

		user, err := m.userRepo.FindByFirebaseUID(r.Context(), decoded.UID)
		if err != nil || user == nil {
			sendError(w, "User not found", "USER_NOT_FOUND", 401)
			return
		}

		ctx := context.WithValue(r.Context(), CtxKeyUser, user)
		ctx = context.WithValue(ctx, CtxKeyFirebase, decoded)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth tries to authenticate but doesn't fail on missing/invalid token.
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r.Header.Get("Authorization"))
		if token != "" {
			if decoded, err := m.firebase.VerifyIDToken(r.Context(), token); err == nil {
				if user, err := m.userRepo.FindByFirebaseUID(r.Context(), decoded.UID); err == nil && user != nil {
					ctx := context.WithValue(r.Context(), CtxKeyUser, user)
					ctx = context.WithValue(ctx, CtxKeyFirebase, decoded)
					r = r.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRegisteredUser requires auth and rejects anonymous users.
func (m *AuthMiddleware) RequireRegisteredUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user != nil && user.IsAnonymous {
				sendError(w, "Anonymous users cannot perform this action", "ANONYMOUS_NOT_ALLOWED", 403)
				return
			}
			next.ServeHTTP(w, r)
		})).ServeHTTP(w, r)
	})
}

// RequireAnonymousUser requires the user to be anonymous (for /auth/convert).
func (m *AuthMiddleware) RequireAnonymousUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user != nil && !user.IsAnonymous {
				sendError(w, "This endpoint is only for anonymous users", "NOT_ANONYMOUS", 403)
				return
			}
			next.ServeHTTP(w, r)
		})).ServeHTTP(w, r)
	})
}

// RequireAdmin checks Firebase custom claim `admin: true` OR static ADMIN_SECRET header.
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check static admin secret header first.
		if m.adminSecret != "" && r.Header.Get("X-Admin-Secret") == m.adminSecret {
			next.ServeHTTP(w, r)
			return
		}
		// Fall back to Firebase token with admin claim.
		token := extractBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			sendError(w, "Admin access required", "FORBIDDEN", 403)
			return
		}
		decoded, err := m.firebase.VerifyIDToken(r.Context(), token)
		if err != nil {
			sendError(w, "Invalid or expired token", "INVALID_TOKEN", 401)
			return
		}
		if admin, _ := decoded.Claims["admin"].(bool); !admin {
			sendError(w, "Admin access required", "FORBIDDEN", 403)
			return
		}
		next.ServeHTTP(w, r)
	})
}
