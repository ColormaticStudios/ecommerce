package handlers

import "ecommerce/models"

func checkoutMode(user *models.User) string {
	if user == nil {
		return "guest"
	}
	return "authenticated"
}

func checkoutUserID(user *models.User) any {
	if user == nil {
		return "none"
	}
	return user.ID
}

func checkoutGuestEmail(email *string) string {
	if email == nil {
		return ""
	}
	return *email
}
