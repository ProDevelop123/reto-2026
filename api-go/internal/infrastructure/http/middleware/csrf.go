package middleware

import (
	"github.com/gofiber/fiber/v3"

	"api-go/pkg/apperror"
)

// CSRFHeaderName es la cabecera que debe acompanar a toda peticion autenticada
// mediante cookie.
const CSRFHeaderName = "X-Refresh-Request"

// RequireCSRFHeader protege los endpoints cuya credencial es una cookie.
//
// # El problema
//
// El refresh token viaja en una cookie, y el navegador las adjunta
// automaticamente a toda peticion hacia su dominio, la origine quien la
// origine. En el despliegue real —frontend en Vercel, APIs en GCP— la cookie
// necesita SameSite=None para poder viajar entre dominios distintos, y eso
// elimina justo la proteccion que SameSite=Lax daba de forma gratuita: un sitio
// cualquiera puede provocar que el navegador de la victima haga POST a
// /auth/refresh con su cookie adjunta.
//
// CORS no lo impide. CORS decide quien puede LEER la respuesta, no quien puede
// ENVIAR la peticion: el servidor la procesa igual y el efecto secundario
// ocurre. Un atacante no podria robar el token, pero si forzar una rotacion, y
// con ella disparar la deteccion de reutilizacion que revoca la familia entera.
// El resultado seria expulsar de su sesion a un usuario legitimo.
//
// # La defensa
//
// Exigir una cabecera propia. Los formularios HTML y las etiquetas img o script
// —los vectores clasicos de CSRF— no pueden anadir cabeceras personalizadas.
// Hacerlo convierte la peticion en "no simple" segun la especificacion de CORS,
// lo que obliga al navegador a lanzar antes una consulta preliminar OPTIONS; y
// esa consulta la rechaza la politica de origenes, que solo admite los de la
// lista. El atacante no llega siquiera a enviar la peticion real.
//
// El valor de la cabecera es irrelevante: lo que protege es que exista, porque
// solo puede ponerla codigo que ya haya superado el control de origen.
func RequireCSRFHeader() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Get(CSRFHeaderName) == "" {
			return apperror.Unauthorized("Falta la cabecera de verificacion de origen.").
				WithDetails(map[string]string{
					"reason": "missing_csrf_header",
					"header": CSRFHeaderName,
				})
		}

		return c.Next()
	}
}
