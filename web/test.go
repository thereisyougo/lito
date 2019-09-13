package web

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"time"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandString(n uint8) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = letters[rand.Intn(len(letters))]
	}
	return string(buf)
}

func CreateTmpScript() (*os.File, error) {
	scriptName := fmt.Sprintf("tmp_script_%s.sh", RandString(9))
	file, err := os.Create(scriptName)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func main() {

	rand.Seed(time.Now().UnixNano())

	script := fmt.Sprintf("tmp_script_%s.sh", RandString(9))
	file, err := os.Create(script)
	defer file.Close()
	defer os.Remove(script)
	failOnError(err, "")

	file.WriteString("cat<<EOF>a.txt\nhello world\nEOF\n")

	cmd := exec.Command("sh", script)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	e := cmd.Run()
	failOnError(e, "")
}

