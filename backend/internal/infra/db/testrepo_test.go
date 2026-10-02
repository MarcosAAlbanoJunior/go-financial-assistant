package db

// testRepo junta o repositório de lançamentos e os de leitura para os testes de integração usarem um só objeto.
type testRepo struct {
	*LedgerRepo
	*DashboardRepo
}
