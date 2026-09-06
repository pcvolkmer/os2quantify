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
	Date            string
	Tumorzellgehalt int64
	OsMolGenId      int64
	// Subforms
	Biomarkers         Biomarkers
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

	// TMB
	if data.Biomarkers.TMB >= 0 {
		result = append(result, "yes", fmt.Sprint(data.Biomarkers.TMB), "")
	} else {
		result = append(result, "no", "", "", "")
	}

	// HRD
	if data.Biomarkers.HRDScore >= 0 {
		result = append(result, "yes", "", fmt.Sprint(data.Biomarkers.HRDScore), "", "")
	} else {
		result = append(result, "no", "", "", "")
	}

	// MSI
	if data.Biomarkers.TMB >= 0 {
		result = append(result, "yes", "", "", fmt.Sprintf("Score: %.2f", data.Biomarkers.TMB))
	} else {
		result = append(result, "no", "", "", "")
	}

	// Tumor cell content
	result = append(result, fmt.Sprintf("%d %%", data.Tumorzellgehalt), "histological", "")

	// up to 10 CSV
	for cnv := range 10 {
		if cnv < len(data.CopyNumberVariants) {
			result = append(result, data.CopyNumberVariants[cnv].Gene, "", data.CopyNumberVariants[cnv].CopyNumber)
		} else {
			result = append(result, "", "", "")
		}
	}

	// up to 20 SV
	for sv := range 20 {
		if sv < len(data.SimpleVariants) {
			result = append(result, data.SimpleVariants[sv].Gene, data.SimpleVariants[sv].AminoAcidChange, data.SimpleVariants[sv].BaseChange, "", data.SimpleVariants[sv].AllelicFraction, "", "", "")
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

type Biomarkers struct {
	TMB      float64
	HRDScore int64
	MSI      float64 // Value, not result -> comment!
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
	query := `SELECT DISTINCT patient.patienten_id, patient.vorname, patient.nachname, patient.geburtsdatum, dk_molekulargenetik.datum, dk_molekulargenetik.tumorzellgehalt, dk_molekulargenetik.id FROM dk_molekulargenetik 
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
	var tumorzellgehalt sql.NullInt64
	var osMolGenId sql.NullInt64

	if rows, err := db.Query(query); err == nil {
		for rows.Next() {
			if err := rows.Scan(&patientenId, &vorname, &nachname, &geburtsdatum, &datum, &tumorzellgehalt, &osMolGenId); err == nil {
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

				if val, err := tumorzellgehalt.Value(); err == nil && val != nil {
					result.Tumorzellgehalt = val.(int64)
				}

				if val, err := osMolGenId.Value(); err == nil && val != nil {
					result.OsMolGenId = val.(int64)
				}

				if biomarkers, err := fetchBiomarkers(db, osMolGenId.Int64); err == nil {
					result.Biomarkers = biomarkers
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
	query := `SELECT DISTINCT untersucht, proteinebenenomenklatur, cdnanomenklatur, allelfrequenz
		FROM dk_molekulargenuntersuchung
		JOIN prozedur ON (prozedur.id = dk_molekulargenuntersuchung.id)
		WHERE prozedur.geloescht <> 1 AND ergebnis = 'P' AND prozedur.hauptprozedur_id = ?
		ORDER BY prozedur.id;`

	var results = []SimpleVariant{}

	var gene sql.NullString
	var aminoAcidChange sql.NullString
	var baseChange sql.NullString
	var allelfrequenz sql.NullString

	if rows, err := db.Query(query, mainFormId); err == nil {
		for rows.Next() {
			if err := rows.Scan(&gene, &aminoAcidChange, &baseChange, &allelfrequenz); err == nil {
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

				if val, err := allelfrequenz.Value(); err == nil && val != nil {
					result.AllelicFraction = val.(string)
				}

				results = append(results, *result)
			}
		}
	}

	return results, nil
}

func fetchBiomarkers(db *sql.DB, mainFormId int64) (Biomarkers, error) {
	query := `SELECT DISTINCT tumormutationalburden, bewertung, komplexerbiomarker
		FROM dk_molekluargenmsi
		JOIN prozedur ON (prozedur.id = dk_molekluargenmsi.id)
		WHERE prozedur.geloescht <> 1 AND komplexerbiomarker = 'TMB' AND prozedur.hauptprozedur_id = ?
		UNION
		SELECT DISTINCT seqprozentwert AS val,  bewertung, komplexerbiomarker
		FROM dk_molekluargenmsi
		JOIN prozedur ON (prozedur.id = dk_molekluargenmsi.id)
		WHERE prozedur.geloescht <> 1 AND komplexerbiomarker = 'MSI' AND prozedur.hauptprozedur_id = ?
		UNION
		SELECT DISTINCT score AS val,  bewertung, komplexerbiomarker
		FROM dk_molekluargenmsi
		JOIN prozedur ON (prozedur.id = dk_molekluargenmsi.id)
		WHERE prozedur.geloescht <> 1 AND komplexerbiomarker = 'HRD' AND prozedur.hauptprozedur_id = ?`

	var value sql.NullFloat64
	var bewertung sql.NullString
	var typ sql.NullString

	result := Biomarkers{}

	if rows, err := db.Query(query, mainFormId, mainFormId, mainFormId); err == nil {
		for rows.Next() {
			if err := rows.Scan(&value, &bewertung, &typ); err == nil {
				if typ, err := typ.Value(); err == nil && typ != nil {
					if typ.(string) == "TMB" {
						if value, err := value.Value(); err == nil && value != nil {
							result.TMB = value.(float64)
						} else {
							result.TMB = -1
						}
					} else if typ.(string) == "MSI" {
						if value, err := value.Value(); err == nil && value != nil {
							result.MSI = value.(float64)
						} else {
							result.MSI = -1
						}
					} else if typ.(string) == "HRD" {
						if value, err := value.Value(); err == nil && value != nil {
							result.HRDScore = int64(value.(float64))
						} else {
							result.HRDScore = -1
						}
					}
				}
			}
		}
	}

	return result, nil
}

func WriteXlsxFile(filename string, patientData []MolGenData) error {
	headers := []string{"PatID",
		"Vorname",
		"Nachname",
		"Geburtstag",
		"Seq. Datum",
		"TMB analysis done",
		"TMB",
		"TMB comment",
		"GI/HRD analysis done",
		"GI/HRD",
		"GI-score",
		"Method",
		"GI/HRD comment",
		"MSI analysis done",
		"MSI result",
		"MSI method",
		"MSI comment",
		"Tumor cell content result",
		"Tumor cell content method",
		"Tumor cell content comment",
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
