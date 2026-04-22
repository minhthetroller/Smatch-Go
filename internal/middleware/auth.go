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
	bpRepo      *repository.BusinessProfileRepository
	adminSecret string
}

func NewAuthMiddleware(fb *firebasepkg.Client, ur *repository.UserRepository, adminSecret string) *AuthMiddleware {
	return &AuthMiddleware{firebase: fb, userRepo: ur, adminSecret: adminSecret}
}

func (m *AuthMiddleware) WithBusinessProfileRepo(repo *repository.BusinessProfileRepository) *AuthMiddleware {
	m.bpRepo = repo
	return m
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

// RequireRole checks that the authenticated user has one of the required roles.
func (m *AuthMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user := UserFromContext(r.Context())
				if user == nil {
					sendError(w, "Unauthorized", "UNAUTHORIZED", 401)
					return
				}
				userRoles, err := m.userRepo.GetRoles(r.Context(), user.ID)
				if err != nil {
					sendError(w, "Failed to check roles", "INTERNAL_ERROR", 500)
					return
				}
				for _, ur := range userRoles {
					for _, rr := range roles {
						if ur == rr {
							next.ServeHTTP(w, r)
							return
						}
					}
				}
				sendError(w, "Insufficient permissions", "FORBIDDEN", 403)
			})).ServeHTTP(w, r)
		})
	}
}

// RequireCourtOwner requires auth, court_owner role, and approved business profile.
func (m *AuthMiddleware) RequireCourtOwner(next http.Handler) http.Handler {
	return m.RequireRole("court_owner")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.bpRepo == nil {
			sendError(w, "Business profile check unavailable", "INTERNAL_ERROR", 500)
			return
		}
		user := UserFromContext(r.Context())
		bp, err := m.bpRepo.FindByUserID(r.Context(), user.ID)
		if err != nil {
			sendError(w, "Failed to check business profile", "INTERNAL_ERROR", 500)
			return
		}
		if bp == nil || bp.Status != "approved" {
			sendError(w, "Business profile not approved", "FORBIDDEN", 403)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// RequireAdmin checks X-Admin-Secret, Firebase admin claim, or admin role.
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check static admin secret header first.
		if m.adminSecret != "" && r.Header.Get("X-Admin-Secret") == m.adminSecret {
			next.ServeHTTP(w, r)
			return
		}
		// Fall back to Firebase token with admin claim or admin role.
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
		if admin, _ := decoded.Claims["admin"].(bool); admin {
			next.ServeHTTP(w, r)
			return
		}
		// Check roles array for admin
		user, _ := m.userRepo.FindByFirebaseUID(r.Context(), decoded.UID)
		if user != nil {
			roles, _ := m.userRepo.GetRoles(r.Context(), user.ID)
			for _, role := range roles {
				if role == "admin" {
					next.ServeHTTP(w, r)
					return
				}
			}
		}
		sendError(w, "Admin access required", "FORBIDDEN", 403)
	})
}
