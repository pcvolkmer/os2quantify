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

