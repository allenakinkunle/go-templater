package templater

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

type ITemplater interface {
	Render(wrt http.ResponseWriter, req *http.Request, templateName string, tmplData interface{})
}

type Templater struct {
	templates map[string]*template.Template
}

func NewTemplater(templateFS fs.FS, rootPath string, functions template.FuncMap) (*Templater, error) {
	templates := map[string]*template.Template{}

	//Walk through the templater embed FS and look for pages
	err := fs.WalkDir(templateFS, ".", func(filePath string, file fs.DirEntry, err error) error {
		if file.IsDir() {
			return nil
		}

		if strings.Contains(file.Name(), ".page.tmpl") {
			tmp, err := template.New(file.Name()).Funcs(functions).ParseFS(templateFS, filePath)

			currentFolder := path.Dir(filePath)
			folders := []string{rootPath, currentFolder}
			// Add the layout and partial templates from the root directory and the directory the page template is in
			for _, folder := range folders {
				err = addFilesToTemplates(templateFS, folder, "*.layout.tmpl", tmp)
				if err != nil {
					return err
				}

				err = addFilesToTemplates(templateFS, folder, "*.partial.tmpl", tmp)
				if err != nil {
					return err
				}
			}

			templateName := strings.TrimSuffix(file.Name(), ".page.tmpl")

			if currentFolder != rootPath {
				templateName = strings.TrimLeft(currentFolder, rootPath) + "/" + strings.TrimSuffix(file.Name(), ".page.tmpl")
			}

			templates[templateName] = tmp
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &Templater{templates: templates}, nil
}

func (t *Templater) Render(wrt http.ResponseWriter, req *http.Request, templateName string, tmplData interface{}) {
	ts, ok := t.templates[templateName]
	if !ok {
		http.Error(wrt, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)

	// Write the template to the buffer, instead of straight to the http Writer
	err := ts.Execute(buf, tmplData)
	if err != nil {
		http.Error(wrt, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Write the contents of the buffer to the http.ResponseWriter
	_, err = buf.WriteTo(wrt)
	if err != nil {
		http.Error(wrt, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func addFilesToTemplates(fsy fs.FS, dirPath, pattern string, tmp *template.Template) error {
	files, err := fs.Glob(fsy, dirPath+"/"+pattern)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	tmp, err = tmp.ParseFS(fsy, files...)
	return err
}
