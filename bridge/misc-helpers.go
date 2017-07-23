package bridge

import (
	"io"
	"os"
	"bufio"
)

func copyFile(in io.Reader, dst string) (err error) {

    // Does file already exist?
    if _, err := os.Stat(dst); err == nil {
        return nil
    }

    err = nil

    out, err := os.Create(dst)
    if err != nil {
        return err
    }

    defer func() {
        cerr := out.Close()
        if err == nil {
            err = cerr
        }
    }()


    
    _, err = io.Copy(out, in)
    if err != nil {
        return err
    }

    err = out.Sync()
    return
}

func readLines(path string) ([]string, error) {
	var lines []string

	file, err := os.Open(path)
	if err != nil {
		// File doesn't exist. Create it.
		file, err = os.Create(path)
		if err != nil {
			return lines, err
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func appendStringToFile(path, text string) error {
      f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
      if err != nil {
              return err
      }
      defer f.Close()

      var text2 = text + "\n"

      _, err = f.WriteString(text2)
      if err != nil {
              return err
      }
      return nil
}