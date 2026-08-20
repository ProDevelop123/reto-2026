package domain

import "time"

// StatisticsRequest es la peticion que se envia al servicio de estadisticas.
type StatisticsRequest struct {
	Matrices  []NamedMatrix
	Tolerance *float64
}

// MatrixStatistics son las metricas de una unica matriz.
type MatrixStatistics struct {
	Name       string
	Rows       int
	Columns    int
	Count      int
	Max        float64
	Min        float64
	Sum        float64
	Average    float64
	IsSquare   bool
	IsDiagonal bool
}

// GlobalStatistics son las metricas agregadas sobre el conjunto de matrices.
// Corresponden literalmente a lo que pide el enunciado: el valor maximo, el
// minimo, el promedio y la suma total encontrados EN LAS MATRICES, mas la
// comprobacion de si alguna de ellas es diagonal.
type GlobalStatistics struct {
	Matrices         int
	Count            int
	Max              float64
	Min              float64
	Sum              float64
	Average          float64
	IsAnyDiagonal    bool
	DiagonalMatrices []string
}

// StatisticsResult es la respuesta completa del servicio de estadisticas.
type StatisticsResult struct {
	Global    GlobalStatistics
	PerMatrix []MatrixStatistics
	// Tolerance es el umbral que el servicio aplico realmente. Se conserva
	// porque el valor de IsDiagonal no es interpretable sin conocerlo.
	Tolerance  float64
	ComputedAt time.Time
}

// Estos tipos describen el concepto de "estadisticas de un conjunto de
// matrices", no el JSON concreto que devuelve la API en Node. La traduccion
// entre ambos es responsabilidad del adaptador, de modo que un cambio en el
// formato de la respuesta del servicio externo no obliga a tocar el dominio ni
// el caso de uso.
