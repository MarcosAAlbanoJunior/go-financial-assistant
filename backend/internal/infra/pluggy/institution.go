package pluggy

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const maxInstitutionNameLen = 60

var hexColor = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

// institution lê o conector do item (GET /items/{id}): nome, cor e logo do banco. Devolve nil
// se a leitura falhar ou o conector não tiver nome. Nada aqui é dado pessoal.
func (c *Client) institution(ctx context.Context, itemID string) *ports.ExternalInstitution {
	var res struct {
		Connector struct {
			Name         string `json:"name"`
			ImageURL     string `json:"imageUrl"`
			PrimaryColor string `json:"primaryColor"`
		} `json:"connector"`
	}
	if err := c.get(ctx, "/items/"+url.PathEscape(itemID), &res); err != nil {
		return nil
	}
	name := strings.Join(strings.Fields(res.Connector.Name), " ")
	if r := []rune(name); len(r) > maxInstitutionNameLen {
		name = string(r[:maxInstitutionNameLen])
	}
	if name == "" {
		return nil
	}
	color := strings.TrimPrefix(strings.TrimSpace(res.Connector.PrimaryColor), "#")
	if !hexColor.MatchString(color) {
		color = ""
	}
	return &ports.ExternalInstitution{Name: name, Color: strings.ToLower(color), ImageURL: res.Connector.ImageURL}
}
