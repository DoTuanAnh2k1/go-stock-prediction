// register_v1.go registers the swagger spec into the v1 swag registry,
// which is what github.com/swaggo/http-swagger/v2 reads from.
package docs

import swagv1 "github.com/swaggo/swag"

// v1wrapper adapts the v2 SwaggerInfo.ReadDoc() to the v1 swag.Swagger interface.
type v1wrapper struct{}

func (v1wrapper) ReadDoc() string {
	return SwaggerInfo.ReadDoc()
}

func init() {
	swagv1.Register(SwaggerInfo.InstanceName(), v1wrapper{})
}
