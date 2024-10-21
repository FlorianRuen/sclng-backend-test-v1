package model

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewAPIError(errReason error) APIError {
	switch errReason.Error() {
	case "RATE_LIMIT_REACHED":
		return APIError{
			Code:    "RATE_LIMIT_REACHED",
			Message: "github rate limit reached. consider using a token to increase the limit or wait few minutes and try again",
		}

	case "RATE_LIMITER_ERROR":
	case "INVALID_DATA_FOUND":
	case "FETCH_ERROR":
		return APIError{
			Code:    errReason.Error(),
			Message: "internal server error. contact our support with the reason code for assistance",
		}

	default:
		return APIError{
			Code:    errReason.Error(),
			Message: "internal server error. contact our support with the reason code for assistance",
		}
	}

	return APIError{
		Code:    "GENERIC_ERROR",
		Message: "internal server error. contact our support with the reason code for assistance",
	}
}
