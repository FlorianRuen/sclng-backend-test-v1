package model

type APIError struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func NewAPIError(err error) APIError {
	
	switch(err.Error()) {
	case "rate limit reached":

	}
	return APIError{
		Reason:  "",
		Message: err.Error(),
	}
}
