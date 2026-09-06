# Onkostar zu BZKF Quantify

Ziel dieser Anwendung ist der Export relevanter Patientendaten und Sequenzierergebnisse aus Onkostar,
damit diese für das BZKF-Projekt "Quantify" verwendet werden können.

## Anwendung

Der Parameter `--help` zeigt folgenden Hilfetext an

```
Usage: os2quantify --user=STRING --filename=STRING [flags]

A simple tool to export data from Onkostar for BZKF Quantify

Flags:
  -h, --help                   Show context-sensitive help.
  -U, --user=STRING            Database username
  -P, --password=STRING        Database password
  -H, --host="localhost"       Database host
      --port=3306              Database port
      --ssl="false"            SSL-Verbindung ('true', 'false', 'skip-verify', 'preferred')
  -D, --database="onkostar"    Database name
      --filename=STRING        Exportiere in diese Datei
```

## Datenquellen

Die generierte Datei enthält Angaben zum Patienten und der molekulargenetischen Untersuchung mit Angaben zu Biomarkern
und Varianten.

### Patient

Die folgenden Angaben werden aus der Tabelle `patient` extrahiert und eingeschlossen, damit der Patient für Quantify
identifiziert werden kann.

| Wert         | Quelltabelle/-Spalte | Beschreibung |
|--------------|----------------------|--------------|
| PatID        | patient.patienten_id | Patienten-ID |
| Vorname      | patient.vorname      | Vorname      |
| Nachname     | patient.nachname     | Nachname     |
| Geburtsdatum | patient.geburtsdatum | Geburtsdatum |

### Molekulargenetische Untersuchung

Jede molekulargenetische Untersuchung wird aus der Tabelle `dk_molekulargenetik` extrahiert und eingeschlossen.

| Wert                      | Quelltabelle/-Spalte                | Beschreibung             |
|---------------------------|-------------------------------------|--------------------------|
| Seq. Datum                | dk_molekulargenetik.datum           | Datum der Sequenzierung  |
| Tumor cell content result | dk_molekulargenetik.tumorzellgehalt | Ergebnis Tumorzellgehalt |
| Tumor cell content method | -                                   | immer "histological"     |

#### Biomarker

Biomarker werden aus der Tabelle `dk_molekluargenmsi` des Unterformulars extrahiert und anhand der Spalte
`dk_molekluargenmsi.komplexerbiomarker` identifiziert.

| Wert     | Quelltabelle/-Spalte                     | Beschreibung                    |
|----------|------------------------------------------|---------------------------------|
| TMB      | dk_molekluargenmsi.tumormutationalburden | Wenn komplexerbiomarker = "TMB" |
| GI-Score | dk_molekluargenmsi.seqprozentwert        | Wenn komplexerbiomarker = "HRD" |
| MSI      | dk_molekluargenmsi.score                 | Wenn komplexerbiomarker = "MSI" |

#### Copy Number Variants

Copy Number Variants werden aus der Tabelle `dk_molekulargenuntersuchung` extrahiert. Hierbei bis zu 10 CNVs.

| Wert        | Quelltabelle/-Spalte                         | Beschreibung     |
|-------------|----------------------------------------------|------------------|
| Gene        | dk_molekulargenuntersuchung.untersucht       | Untersuchtes Gen |
| Fold Change | -                                            | Leer             |
| Copy Number | dk_molekulargenuntersuchung.cnvtotalcndouble | Kopienzahl       |

#### Einfache Varianten

Weitere Mutationen werden ebenfalls aus der Tabelle `dk_molekulargenuntersuchung` extrahiert. Hierbei bis zu 20 SVs.

| Wert                  | Quelltabelle/-Spalte                                | Beschreibung     |
|-----------------------|-----------------------------------------------------|------------------|
| Gene                  | dk_molekulargenuntersuchung.untersucht              | Untersuchtes Gen |
| Amino Acid change     | dk_molekulargenuntersuchung.proteinebenenomenklatur | Proteinabene     |                                            | Leer             |
| Base change           | dk_molekulargenuntersuchung.cdnanomenklatur         | cDNA             |
| Clinical significance | -                                                   | Leer             |
| Allelic fraction(%)   | dk_molekulargenuntersuchung.allelfrequenz           | Allelfrequenz    |                  |
| Classification        | -                                                   | Leer             |
| Impact                | -                                                   | Leer             |

#### Fusionen

Stehen noch aus.