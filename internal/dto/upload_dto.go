package dto

type ImageUploadResponse struct {
	Key      string `json:"key"`
	URL      string `json:"url"`
	FileName string `json:"fileName"`
}

type ProfilePhotoUploadResponse struct {
	User UserProfileResponse `json:"user"`
}
