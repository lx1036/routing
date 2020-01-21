package auth

import (
	"k8s-lx1036/k8s-ui/backend/models"
)

type DBAuth struct{}

func (auth DBAuth) Authenticate(model models.AuthModel) (*models.User, error) {
	user, err := models.UserModel.GetUserByName(model.Username)

	return user, err
}

func init() {
	Register(models.AuthTypeDB, &DBAuth{})
}