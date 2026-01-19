package main

import (
	"bytes"
	"fmt"
	"flag"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"github.com/labstack/echo/v4"
	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"
)

type LivelliLingua struct {
	Lingua          string `yaml:"lingua"`
	Ascolto         string `yaml:"ascolto"`
	Lettura         string `yaml:"lettura"`
	Interazione     string `yaml:"interazione"`
	ProduzioneOrale string `yaml:"produzione_orale"`
	Scrittura       string `yaml:"scrittura"`
}

// La struttura che ospiterà i tuoi dati
type CVData struct {
	Nome           string              `yaml:"nome"`
	Cognome	       string              `yaml:"cognome"`
	DataNascita    string              `yaml:"data_nascita"`
	Sesso		   string              `yaml:"sesso"`
	Posizione      string              `yaml:"posizione"`
	Abitazione     string              `yaml:"abitazione"`
	Email          string              `yaml:"email"`
	Telefono       string              `yaml:"telefono"`
	GitHub         string              `yaml:"github"`
	Linkedin       string              `yaml:"linkedin"`
	Nazionalita    string              `yaml:"nazionalita"`
	TopSkills      []string            `yaml:"top_skills"`     // Quelle nell'header
	LinguaMadre    string              `yaml:"lingua_madre"`
	Lingue         []LivelliLingua     `yaml:"lingue"`
	LingueSito     []string            `yaml:"lingueSito"`
	SkillsPerArea  map[string][]string `yaml:"skills_per_area"` // Skill categorizzate
	Content        template.HTML
}

// Funzione per leggere e processare il file MD
func loadCV() (CVData, error) {
	file, err := os.ReadFile("data/cv.md")
	if err != nil {
		return CVData{}, err
	}

	parts := strings.SplitN(string(file), "---", 3)
	
	var data CVData
	yaml.Unmarshal([]byte(parts[1]), &data)

	var buf bytes.Buffer
	goldmark.Convert([]byte(parts[2]), &buf)
	data.Content = template.HTML(buf.String())

	return data, nil
}

func (c CVData) NomeCompleto() string {
    return c.Nome + " " + c.Cognome
}

// Renderer per Echo
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
	// 1. Definiamo il flag -export
	exportMode := flag.Bool("export", false, "Esegue l'esportazione statica e termina")
	flag.Parse()

	// 2. Se il flag è presente, eseguiamo l'export e USCIAMO
	if *exportMode {
		exportStatic()
		return
	}

	// 3. Altrimenti, avviamo il server normalmente per lo sviluppo locale
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
		if err != nil { return err }
		return c.Render(http.StatusOK, "index.html", cv)
	})

	e.GET("/pdf", func(c echo.Context) error {
		cv, err := loadCV()
		if err != nil { return err }
		return c.Render(http.StatusOK, "pdf.html", cv)
	})

	e.Logger.Fatal(e.Start(":8080"))
}