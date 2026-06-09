package request

type Login struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	//AuthCode string `json:"authCode" binding:"authCode"`
	//AuCodeID int    `json:"auCodeID"`
}
