package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

// genFixtures пишет синтетический датасет Астаны в dir (contract/lots/trd-buy/rnu .json),
// чтобы разработка и аудит шли без токена. Данные детерминированы (фиксированный seed).
// Имена полей совпадают с каноническими кандидатами из models.go.
func genFixtures(dir string, n int) error {
	if n <= 0 {
		n = 300
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	rng := rand.New(rand.NewSource(42))

	streets := []string{
		"проспект Кабанбай батыра", "улица Кенесары", "проспект Мангилик Ел", "улица Сыганак",
		"проспект Туран", "улица Достык", "проспект Республики", "улица Сарайшык",
		"улица Айтеке би", "проспект Богенбай батыра", "улица Жирентаева", "улица Кошкарбаева",
	}
	roadTmpl := []string{
		"Капитальный ремонт автодороги по %s",
		"Средний ремонт дорожного покрытия, %s",
		"Реконструкция проезжей части по %s",
	}
	waterTmpl := []string{
		"Капитальный ремонт сетей водоснабжения по %s",
		"Строительство водопровода, %s",
		"Реконструкция канализационных сетей по %s",
	}
	otherTmpl := []string{
		"Поставка канцелярских товаров для %s",
		"Услуги охраны объектов %s",
	}
	const astanaKATO = "710000000"

	bin := func() string {
		s := ""
		for i := 0; i < 12; i++ {
			s += fmt.Sprintf("%d", rng.Intn(10))
		}
		return s
	}
	suppliers := make([]string, 8) // пул подрядчиков (чтобы возникали монополия/повторы)
	for i := range suppliers {
		suppliers[i] = bin()
	}
	customers := []string{bin(), bin(), bin()} // управления акимата

	var contracts, lots, trdbuy, rnu []map[string]any
	for i := 0; i < n; i++ {
		anno := fmt.Sprintf("AST-%06d", 100000+i)
		street := streets[rng.Intn(len(streets))]
		var name string
		switch roll := rng.Intn(10); {
		case roll < 5:
			name = fmt.Sprintf(roadTmpl[rng.Intn(len(roadTmpl))], street)
		case roll < 8:
			name = fmt.Sprintf(waterTmpl[rng.Intn(len(waterTmpl))], street)
		default:
			name = fmt.Sprintf(otherTmpl[rng.Intn(len(otherTmpl))], "ГУ Астаны")
		}
		sup := suppliers[rng.Intn(len(suppliers))]
		if rng.Intn(20) == 0 { // иногда «битый» БИН — проверка нормализации
			sup = ""
		}
		cust := customers[rng.Intn(len(customers))]
		sum := float64(rng.Intn(900)+100) * 1_000_000 // 100M..1B ₸
		signDate := fmt.Sprintf("2025-%02d-%02d", rng.Intn(12)+1, rng.Intn(27)+1)
		participants := 1
		if rng.Intn(3) != 0 {
			participants = rng.Intn(4) + 2
		}

		trdbuy = append(trdbuy, map[string]any{
			"number_anno": anno, "name_ru": name, "org_bin": cust,
			"total_sum": sum, "publish_date": signDate, "count": participants,
		})
		lots = append(lots, map[string]any{
			"trd_buy_number_anno": anno, "name_ru": name, "amount": sum,
			"customer_bin": cust, "ref_kato": astanaKATO,
		})
		contracts = append(contracts, map[string]any{
			"trd_buy_number_anno": anno, "supplier_biin": sup, "customer_bin": cust,
			"contract_sum_wnds": sum, "sign_date": signDate,
		})
	}
	for _, b := range []string{suppliers[0], suppliers[1]} { // пара подрядчиков в РНУ
		rnu = append(rnu, map[string]any{
			"supplier_biin": b, "start_date": "2025-01-01", "end_date": "",
		})
	}

	write := func(res string, v any) error {
		data, _ := json.MarshalIndent(v, "", "  ")
		return os.WriteFile(filepath.Join(dir, res+".json"), data, 0o644)
	}
	for res, v := range map[string]any{"contract": contracts, "lots": lots, "trd-buy": trdbuy, "rnu": rnu} {
		if err := write(res, v); err != nil {
			return err
		}
	}
	fmt.Printf("Фикстуры записаны в %s: contracts=%d lots=%d trd-buy=%d rnu=%d\n",
		dir, len(contracts), len(lots), len(trdbuy), len(rnu))
	return nil
}
