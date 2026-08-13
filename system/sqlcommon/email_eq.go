package sqlcommon

import (
	"strings"

	"github.com/apito-io/engine/models"
)

// EmailEqFromParams returns where.email.eq from a system search param, or empty.
func EmailEqFromParams(param *models.CommonSystemParams) string {
	if param == nil || param.ResolveParams == nil {
		return ""
	}
	where, ok := param.ResolveParams.Args["where"].(map[string]interface{})
	if !ok {
		return ""
	}
	emailField, ok := where["email"].(map[string]interface{})
	if !ok {
		return ""
	}
	v, ok := emailField["eq"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}
