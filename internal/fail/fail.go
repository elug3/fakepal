package fail

import (
	"encoding/json"
	"strings"

	"github.com/elug3/fakepal/internal/domain"
)

const HeaderMockResponse = "PayPal-Mock-Response"

// Decision is the result of evaluating failure injection rules.
type Decision struct {
	Deny  bool
	Error *domain.ErrorResponse
}

// Evaluate inspects mock headers and amount heuristics.
// Amount ending in ".13" triggers INSTRUMENT_DECLINED for create/capture.
func Evaluate(mockHeader string, amount domain.Amount) Decision {
	if mockHeader != "" {
		var payload struct {
			MockApplicationCodes string `json:"mock_application_codes"`
		}
		if err := json.Unmarshal([]byte(mockHeader), &payload); err == nil && payload.MockApplicationCodes != "" {
			code := payload.MockApplicationCodes
			return Decision{
				Deny: true,
				Error: &domain.ErrorResponse{
					Name:    "UNPROCESSABLE_ENTITY",
					Message: "The requested action could not be performed, semantically incorrect, or failed business validation.",
					Details: []domain.ErrorDetail{{
						Issue:       code,
						Description: "Injected mock failure: " + code,
					}},
				},
			}
		}
		// Also allow a bare code string in the header.
		code := strings.TrimSpace(mockHeader)
		if code != "" && !strings.HasPrefix(code, "{") {
			return Decision{
				Deny: true,
				Error: &domain.ErrorResponse{
					Name:    "UNPROCESSABLE_ENTITY",
					Message: "The requested action could not be performed, semantically incorrect, or failed business validation.",
					Details: []domain.ErrorDetail{{
						Issue:       code,
						Description: "Injected mock failure: " + code,
					}},
				},
			}
		}
	}

	if strings.HasSuffix(strings.TrimSpace(amount.Value), ".13") {
		return Decision{
			Deny: true,
			Error: &domain.ErrorResponse{
				Name:    "UNPROCESSABLE_ENTITY",
				Message: "The requested action could not be performed, semantically incorrect, or failed business validation.",
				Details: []domain.ErrorDetail{{
					Issue:       "INSTRUMENT_DECLINED",
					Description: "The instrument presented was declined (mock heuristic .13).",
				}},
			},
		}
	}

	return Decision{}
}
