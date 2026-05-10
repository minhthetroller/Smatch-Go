package dto

type ImageUploadResponse struct {
	URL      string `json:"url"`
	FileName string `json:"fileName"`
}

type ProfilePhotoUploadResponse struct {
	User UserProfileResponse `json:"user"`
}
