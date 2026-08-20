package domain

import "errors"

// Errores del dominio.
//
// Se declaran como valores centinela para que las capas superiores los
// distingan con errors.Is y decidan el codigo HTTP correspondiente, en lugar
// de comparar mensajes de texto. El dominio expresa QUE ha fallado; traducirlo
// a un codigo de estado es responsabilidad del borde HTTP.
var (
	// ErrSessionNotFound indica que el refresh token presentado no corresponde
	// a ninguna sesion registrada.
	ErrSessionNotFound = errors.New("sesion no encontrada")

	// ErrInvalidCredentials indica un usuario o contrasena incorrectos.
	//
	// Es un unico error para ambos casos a proposito: distinguir "el usuario no
	// existe" de "la contrasena es incorrecta" permitiria a un atacante
	// enumerar usuarios validos.
	ErrInvalidCredentials = errors.New("credenciales invalidas")

	// ErrSessionExpired indica que el refresh token ha caducado.
	ErrSessionExpired = errors.New("la sesion ha expirado")

	// ErrSessionReused indica que se ha presentado un refresh token que ya
	// habia sido canjeado. Es la firma de un token robado: el legitimo ya lo
	// uso, asi que quien lo presenta ahora obtuvo una copia.
	ErrSessionReused = errors.New("el token de refresco ya fue utilizado")
)
