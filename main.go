package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

// Convertitore Markdown configurato correttamente
var mdConverter = goldmark.New(
	goldmark.WithRendererOptions(
		html.WithHardWraps(), // Questo è il modo corretto per abilitare gli "a capo"
		html.WithUnsafe(),   // Permette l'uso di HTML grezzo nel Markdown
	),
)

// --- STRUCTS ---

type LivelliLingua struct {
	Lingua          string `yaml:"lingua"`
	Ascolto         string `yaml:"ascolto"`
	Lettura         string `yaml:"lettura"`
	Interazione     string `yaml:"interazione"`
	ProduzioneOrale string `yaml:"produzione_orale"`
	Scrittura       string `yaml:"scrittura"`
}

type Esperienza struct {
	Qualifica     string        `yaml:"qualifica"`
	DatoreLavoro  string        `yaml:"datore_lavoro"`
	Periodo       string        `yaml:"periodo"`
	Luogo         string        `yaml:"luogo"`
	Descrizione   template.HTML `yaml:"-"`
	DescrizioneMD string        `yaml:"descrizione"`
}

type Istruzione struct {
	Titolo        string        `yaml:"titolo"`
	Istituto      string        `yaml:"istituto"`
	Periodo       string        `yaml:"periodo"`
	Luogo         string        `yaml:"luogo"`
	Descrizione   template.HTML `yaml:"-"`
	DescrizioneMD string        `yaml:"descrizione"`
}

type CVData struct {
	Nome            string                   `yaml:"nome"`
	Cognome         string                   `yaml:"cognome"`
	DataNascita     string                   `yaml:"dataNascita"`
	Sesso           string                   `yaml:"sesso"`
	Posizione       string                   `yaml:"posizione"`
	Abitazione      string                   `yaml:"abitazione"`
	Email           string                   `yaml:"email"`
	Telefono        string                   `yaml:"telefono"`
	GitHub          string                   `yaml:"github"`
	Linkedin        string                   `yaml:"linkedin"`
	Nazionalita     string                   `yaml:"nazionalita"`
	TopSkills       []string                 `yaml:"top_skills"`
	LinguaMadre     string                   `yaml:"lingua_madre"`
	Lingue          []LivelliLingua          `yaml:"lingue"`
	LingueSito      []string                 `yaml:"lingueSito"`
	SkillsPerArea   map[string][]string      `yaml:"skills_per_area"`
	Esperienze      []Esperienza             `yaml:"esperienze"`
	Istruzione      []Istruzione             `yaml:"istruzione"`
	SezioniMarkdown map[string]template.HTML `yaml:"-"`
}

// --- LOGICA DI CARICAMENTO ---

func loadCV() (CVData, error) {
	file, err := os.ReadFile("data/cv.md")
	if err != nil {
		return CVData{}, err
	}

	parts := strings.SplitN(string(file), "---", 3)
	if len(parts) < 3 {
		return CVData{}, fmt.Errorf("file 'data/cv.md' non contiene un front matter YAML valido racchiuso da '---'")
	}

	var data CVData
	if err := yaml.Unmarshal([]byte(parts[1]), &data); err != nil {
		return CVData{}, fmt.Errorf("errore nel parsing del YAML: %w", err)
	}

	mdToHTML := func(md string) template.HTML {
		var buf bytes.Buffer
		if err := mdConverter.Convert([]byte(md), &buf); err != nil {
			fmt.Printf("Errore conversione Markdown: %v\n", err)
			return template.HTML("Errore conversione Markdown")
		}
		return template.HTML(buf.String())
	}

	for i := range data.Esperienze {
		data.Esperienze[i].Descrizione = mdToHTML(data.Esperienze[i].DescrizioneMD)
	}

	for i := range data.Istruzione {
		data.Istruzione[i].Descrizione = mdToHTML(data.Istruzione[i].DescrizioneMD)
	}

	data.SezioniMarkdown = make(map[string]template.HTML)
	markdownBody := strings.TrimSpace(parts[2])

	if markdownBody != "" {
		const delimiter = "\n## "
		if !strings.HasPrefix(markdownBody, "## ") {
			lines := strings.SplitN(markdownBody, delimiter, 2)
			data.SezioniMarkdown["Principale"] = mdToHTML(lines[0])
			if len(lines) > 1 {
				markdownBody = lines[1]
			} else {
				markdownBody = ""
			}
		}

		if strings.HasPrefix(markdownBody, "## ") {
			markdownBody = strings.TrimPrefix(markdownBody, "## ")
		}

		sections := strings.Split(markdownBody, delimiter)
		for _, section := range sections {
			section = strings.TrimSpace(section)
			if section == "" {
				continue
			}

			lines := strings.SplitN(section, "\n", 2)
			title := strings.TrimSpace(lines[0])
			content := ""
			if len(lines) > 1 {
				content = lines[1]
			}

			if title != "" {
				data.SezioniMarkdown[title] = mdToHTML(content)
			}
		}
	}

	// Debug: Stampa la struttura dati in formato JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println("Errore nel marshalling JSON per il debug:", err)
	} else {
		fmt.Println("--- INIZIO DEBUG DATI CV ---")
		fmt.Println(string(jsonData))
		fmt.Println("--- FINE DEBUG DATI CV ---")
	}

	return data, nil
}

func (c CVData) NomeCompleto() string {
	return c.Nome + " " + c.Cognome
}

// --- RESTO DEL FILE ---

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func exportStatic() {
	fmt.Println("🚀 Avvio esportazione statica...")
	cv, err := loadCV()
	if err != nil {
		fmt.Printf("❌ Errore caricamento dati: %v\n", err)
		return
	}

	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}).ParseGlob("templates/*.html"))

	os.MkdirAll("dist/static", 0755)

	fIndex, _ := os.Create("dist/index.html")
	tmpl.ExecuteTemplate(fIndex, "index.html", cv)
	fIndex.Close()

	fPDF, _ := os.Create("dist/pdf.html")
	tmpl.ExecuteTemplate(fPDF, "pdf.html", cv)
	fPDF.Close()

	err = exec.Command("cp", "-r", "static/.", "dist/static/").Run()
	if err != nil {
		fmt.Printf("⚠️ Nota: Copia statici fallita (normale su Windows): %v\n", err)
	}
	fmt.Println("✅ Esportazione completata nella cartella /dist")
}

func main() {
	exportMode := flag.Bool("export", false, "Esegue l'esportazione statica e termina")
	flag.Parse()

	if *exportMode {
		exportStatic()
		return
	}

	e := echo.New()

	renderer := &TemplateRenderer{
		templates: template.New("").Funcs(template.FuncMap{
			"add": func(a, b int) int { return a + b },
		}),
	}

	_, err := renderer.templates.ParseGlob("templates/*.html")
	if err != nil {
		e.Logger.Fatal("Errore caricamento template:", err)
	}

	e.Renderer = renderer
	e.Static("/static", "static")

	e.GET("/", func(c echo.Context) error {
		cv, err := loadCV()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.Render(http.StatusOK, "index.html", cv)
	})

	e.GET("/pdf", func(c echo.Context) error {
		cv, err := loadCV()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.Render(http.StatusOK, "pdf.html", cv)
	})

	e.Logger.Fatal(e.Start(":8080"))
}
