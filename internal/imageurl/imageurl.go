package imageurl

import "strings"

// Resolver builds public S3 URLs from stored object keys.
// It is tolerant: values that already look like full URLs (http://, https://)
// are returned as-is, so existing DB rows with full URLs need no migration.
type Resolver struct {
	matchesBase string
	profileBase string
}

// New creates a resolver with the given public base URLs.
// Trailing slashes on base URLs are normalized away.
func New(matchesBase, profileBase string) Resolver {
	return Resolver{
		matchesBase: strings.TrimRight(matchesBase, "/"),
		profileBase: strings.TrimRight(profileBase, "/"),
	}
}

// Match resolves a match-image key to a public URL.
func (r Resolver) Match(key string) string {
	return resolve(r.matchesBase, key)
}

// Profile resolves a profile-photo key to a public URL.
func (r Resolver) Profile(key string) string {
	return resolve(r.profileBase, key)
}

func resolve(base, key string) string {
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	return base + "/" + key
}
