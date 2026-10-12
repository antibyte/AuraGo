package gog

import "aurago/internal/config"

// Some Office CLI commands use Drive in addition to their document API.
func additionalGrants(req Request) map[string]string {
	grants := map[string]string{}
	switch {
	case (req.Service == "docs" || req.Service == "sheets" || req.Service == "slides") && req.Operation == "export":
		grants["drive"] = "read"
	case req.Service == "docs" && req.Operation == "create":
		grants["drive"] = "create"
	case req.Service == "slides" && req.Operation == "create":
		grants["drive"] = "create"
	case req.Service == "sheets" && req.Operation == "create" && req.Args["parent"] != nil:
		grants["drive"] = "update"
	case req.Service == "drive" && req.Operation == "import_office":
		target, _ := req.Args["convert_to"].(string)
		if service := map[string]string{"doc": "docs", "sheet": "sheets", "slides": "slides"}[target]; service != "" {
			grants[service] = "create"
		}
	}
	return grants
}

func authorizeOperation(cfg *config.Config, source string, req Request, grant string) (config.GogAccount, error) {
	account, err := Authorize(cfg, source, req, grant)
	if err != nil {
		return account, err
	}
	for service, g := range additionalGrants(req) {
		next := req
		next.Service = service
		if _, err = Authorize(cfg, source, next, g); err != nil {
			return account, err
		}
	}
	return account, nil
}
