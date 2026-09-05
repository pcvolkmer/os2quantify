package main

import (
	"database/sql"
	"fmt"
	"log"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/go-sql-driver/mysql"
	"github.com/xuri/excelize/v2"
	"golang.org/x/term"
)

var (
	cli     *CLI
	context *kong.Context
	db      *sql.DB
)

type CLI struct {
	User     string `short:"U" help:"Database username" required:"NA"`
	Password string `short:"P" help:"Database password"`
	Host     string `short:"H" help:"Database host" default:"localhost"`
	Port     int    `help:"Database port" default:"3306"`
	Ssl      string `help:"SSL-Verbindung ('true', 'false', 'skip-verify', 'preferred')" default:"false"`
	Database string `short:"D" help:"Database name" default:"onkostar"`
	Filename string `help:"Exportiere in diese Datei" required:"NA"`
}

type MolGenData struct {
	// From: Patient
	PatientID string
	FirstName string
	LastName  string
	Birthdate string
	// From: OS.Molekulargenetik
	Date               string
	OsMolGenId         int64
	CopyNumberVariants []CopyNumberVariant
	SimpleVariants     []SimpleVariant
}

func (data *MolGenData) AsStringArray() []string {
	result := []string{
		data.PatientID,
		data.FirstName,
		data.LastName,
		data.Birthdate,
		data.Date,
	}

	for cnv := range 10 {
		if cnv < len(data.CopyNumberVariants) {
			result = append(result, data.CopyNumberVariants[cnv].Gene, "", data.CopyNumberVariants[cnv].CopyNumber)
		} else {
			result = append(result, "", "", "")
		}
	}

	for sv := range 20 {
		if sv < len(data.SimpleVariants) {
			result = append(result, data.SimpleVariants[sv].Gene, data.SimpleVariants[sv].AminoAcidChange, data.SimpleVariants[sv].BaseChange, "", "", "", "", "")
		} else {
			result = append(result, "", "", "", "", "", "", "", "")
		}
	}

	return result
}

type CopyNumberVariant struct {
	Gene       string
	CopyNumber string
}

type SimpleVariant struct {
	Gene                 string
	AminoAcidChange      string
	BaseChange           string
	ClinicalSignificance string
	AllelicFraction      string
	Classification       string
	Function             string
	Impact               string
}

func initCLI() {
	cli = &CLI{}
	context = kong.Parse(cli,
		kong.Name("os2quantify"),
		kong.Description("A simple tool to export data from Onkostar for BZKF Quantify"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
	)
}

func initDb(dbCfg mysql.Config) (*sql.DB, error) {
	if dbx, err := sql.Open("mysql", dbCfg.FormatDSN()); err == nil {
		if err := dbx.Ping(); err == nil {
			return dbx, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func fetchMolGens(db *sql.DB) ([]MolGenData, error) {
	query := `SELECT DISTINCT patient.patienten_id, patient.vorname, patient.nachname, patient.geburtsdatum, dk_molekulargenetik.datum, dk_molekulargenetik.id FROM dk_molekulargenetik 
		JOIN prozedur ON (prozedur.id = dk_molekulargenetik.id)
		JOIN patient ON (patient.id = prozedur.patient_id)
		WHERE prozedur.geloescht <> 1
		ORDER BY dk_molekulargenetik.datum, patienten_id;`

	var results = []MolGenData{}

	var patientenId sql.NullString
	var vorname sql.NullString
	var nachname sql.NullString
	var geburtsdatum sql.NullString
	// Main Form "OS.Molekulargenetik" with subforms for variants and biomarkers
	var datum sql.NullString
	var osMolGenId sql.NullInt64

	if rows, err := db.Query(query); err == nil {
		for rows.Next() {
			if err := rows.Scan(&patientenId, &vorname, &nachname, &geburtsdatum, &datum, &osMolGenId); err == nil {
				result := &MolGenData{}

				if val, err := patientenId.Value(); err == nil && val != nil {
					result.PatientID = val.(string)
				}

				if val, err := vorname.Value(); err == nil && val != nil {
					result.FirstName = val.(string)
				}

				if val, err := nachname.Value(); err == nil && val != nil {
					result.LastName = val.(string)
				}

				if val, err := geburtsdatum.Value(); err == nil && val != nil {
					result.Birthdate = val.(string)
				}

				if val, err := datum.Value(); err == nil && val != nil {
					result.Date = val.(string)
				}

				if val, err := osMolGenId.Value(); err == nil && val != nil {
					result.OsMolGenId = val.(int64)
				}

				if copyNumberVariants, err := fetchCnvs(db, osMolGenId.Int64); err == nil {
					result.CopyNumberVariants = copyNumberVariants[:min(10, len(copyNumberVariants))]
				}

				if simpleVariants, err := fetchSvs(db, osMolGenId.Int64); err == nil {
					result.SimpleVariants = simpleVariants[:min(20, len(simpleVariants))]
				}

				results = append(results, *result)
			}
		}
	}

	return results, nil
}

func fetchCnvs(db *sql.DB, mainFormId int64) ([]CopyNumberVariant, error) {
	query := `SELECT DISTINCT untersucht, cnvtotalcndouble
		FROM dk_molekulargenuntersuchung
		JOIN prozedur ON (prozedur.id = dk_molekulargenuntersuchung.id)
		WHERE prozedur.geloescht <> 1 AND ergebnis = 'CNV' AND prozedur.hauptprozedur_id = ?
		ORDER BY prozedur.id;`

	var results = []CopyNumberVariant{}

	var gene sql.NullString
	var copyNumber sql.NullFloat64

	if rows, err := db.Query(query, mainFormId); err == nil {
		for rows.Next() {
			if err := rows.Scan(&gene, &copyNumber); err == nil {
				result := &CopyNumberVariant{}

				if val, err := gene.Value(); err == nil && val != nil {
					result.Gene = val.(string)
				}

				if val, err := copyNumber.Value(); err == nil && val != nil {
					result.CopyNumber = fmt.Sprintf("%.2f", val.(float64))
				}

				results = append(results, *result)
			}
		}
	}

	return results, nil
}

func fetchSvs(db *sql.DB, mainFormId int64) ([]SimpleVariant, error) {
	query := `SELECT DISTINCT untersucht, proteinebenenomenklatur, cdnanomenklatur
		FROM dk_molekulargenuntersuchung
		JOIN prozedur ON (prozedur.id = dk_molekulargenuntersuchung.id)
		WHERE prozedur.geloescht <> 1 AND ergebnis = 'P' AND prozedur.hauptprozedur_id = ?
		ORDER BY prozedur.id;`

	var results = []SimpleVariant{}

	var gene sql.NullString
	var aminoAcidChange sql.NullString
	var baseChange sql.NullString

	if rows, err := db.Query(query, mainFormId); err == nil {
		for rows.Next() {
			if err := rows.Scan(&gene, &aminoAcidChange, &baseChange); err == nil {
				result := &SimpleVariant{}

				if val, err := gene.Value(); err == nil && val != nil {
					result.Gene = val.(string)
				}

				if val, err := aminoAcidChange.Value(); err == nil && val != nil {
					result.AminoAcidChange = val.(string)
				}

				if val, err := baseChange.Value(); err == nil && val != nil {
					result.BaseChange = val.(string)
				}

				results = append(results, *result)
			}
		}
	}

	return results, nil
}

func WriteXlsxFile(filename string, patientData []MolGenData) error {
	headers := []string{"PatID",
		"Vorname",
		"Nachname",
		"Geburtstag",
		"Seq. Datum",
	}

	for i := range 10 {
		headers = append(headers, fmt.Sprintf("CNV %d: Gene", i+1))
		headers = append(headers, fmt.Sprintf("CNV %d: Fold Change", i+1))
		headers = append(headers, fmt.Sprintf("CNV %d: Copy Number", i+1))
	}

	for i := range 20 {
		headers = append(headers, fmt.Sprintf("Mutation %d: Gene", i+1))
		headers = append(headers, fmt.Sprintf("Mutation %d: Amino Acid change", i+1))
		headers = append(headers, fmt.Sprintf("Mutation %d: Base change", i+1))
		headers = append(headers, fmt.Sprintf("Mutation %d: Clinical significance", i+1))
		headers = append(headers, fmt.Sprintf("Mutation %d: Allelic fraction(%%)", i+1))
		headers = append(headers, fmt.Sprintf("Mutation %d: Classification", i+1))
		headers = append(headers, fmt.Sprintf("Mutation %d: Function", i+1))
		headers = append(headers, fmt.Sprintf("Mutation %d: Impact", i+1))
	}

	file := excelize.NewFile()
	defer func() {
		if err := file.Close(); err != nil {
			return
		}
	}()

	index, _ := file.NewSheet("Quantify excerpt")
	file.SetActiveSheet(index)

	for idx, columnHeader := range headers {
		cell := getExcelColumn(idx) + "1"
		_ = file.SetCellValue("Quantify excerpt", cell, columnHeader)
	}

	for row, data := range patientData {
		for idx, value := range data.AsStringArray() {
			cell := getExcelColumn(idx) + fmt.Sprint(row+2)
			_ = file.SetCellValue("Quantify excerpt", cell, value)
		}
	}

	_ = file.DeleteSheet("Sheet1")

	if err := file.SaveAs(filename); err != nil {
		fmt.Println(err)
	}

	return nil
}

func getExcelColumn(idx int) string {
	z := int('Z' - 'A' + 1)
	m := idx % z
	if idx <= m {
		return string(rune(idx + 'A'))
	}

	r := ((idx - m) / z) - 1
	return string(rune(r+'A')) + string(rune(m+'A'))
}

func main() {

	initCLI()

	if len(cli.Password) == 0 {
		fmt.Print("Passwort: ")
		if bytePw, err := term.ReadPassword(int(syscall.Stdin)); err == nil {
			cli.Password = string(bytePw)
		}
		println()
	}

	dbCfg := mysql.Config{
		User:                 cli.User,
		Passwd:               cli.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", cli.Host, cli.Port),
		DBName:               cli.Database,
		AllowNativePasswords: true,
		TLSConfig:            cli.Ssl,
	}

	if dbx, dbErr := initDb(dbCfg); dbErr == nil {
		db = dbx
		defer func(db *sql.DB) {
			err := db.Close()
			if err != nil {
				log.Println("Cannot close database connection")
			}
		}(db)
	} else {
		log.Fatalf("Cannot connect to Database: %s\n", dbErr.Error())
	}

	patients, err := fetchMolGens(db)
	if err != nil {
		log.Fatalf("Cannot fetch patients: %s\n", err.Error())
	}

	WriteXlsxFile(cli.Filename, patients)

}
