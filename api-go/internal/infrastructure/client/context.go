package client

import "context"

// contextKey es un tipo privado para las claves de contexto.
//
// Se define un tipo propio en lugar de usar una cadena suelta porque las
// claves de contexto viven en un espacio de nombres global compartido por todo
// el proceso: con cadenas, dos paquetes que eligieran el mismo texto se
// pisarian los valores sin previo aviso. Un tipo no exportado hace imposible
// esa colision.
type contextKey struct{ name string }

var accessTokenKey = contextKey{"access-token"}

// WithAccessToken adjunta al contexto el token de acceso del usuario para que
// el adaptador lo propague al servicio de estadisticas.
//
// La identidad viaja por el contexto y no como parametro del puerto de forma
// deliberada: el caso de uso no debe conocer los detalles de autenticacion del
// transporte. Para el nucleo, "analizar matrices" no lleva credenciales; que la
// llamada concreta necesite un token es un asunto del adaptador.
func WithAccessToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, accessTokenKey, token)
}
