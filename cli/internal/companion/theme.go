package companion

import (
 "fmt"
 "io"
 "os"
 "strings"
 "time"
)

// RunTerminal adds presentation only. Run remains plain for programmatic callers.
func RunTerminal(args []string, invoked string, out, errOut *os.File) int {
 info, err := out.Stat()
 _, noColor := os.LookupEnv("NO_COLOR")
 term := os.Getenv("TERM")
 interactive := err == nil && info.Mode()&os.ModeCharDevice != 0 && term != "" && term != "dumb"
 show := len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h"))
 for _, arg := range args {
  if arg == "--json" { show = false }
 }
 if interactive && !noColor && show {
  banner(out, os.Getenv("OMNI_ANIMATE") == "1")
 }
 return Run(args, invoked, out, errOut)
}

// Use the terminal background; never change the user's palette or startup files.
func banner(w io.Writer, animate bool) {
 const gold = "\x1b[33m"
 const reset = "\x1b[0m"
 if animate {
  for _, frame := range []string{"o . . .", ". o . .", ". . o .", ". . . o"} {
   fmt.Fprintf(w, "\r%sOMNI ORBIT  %s%s", gold, frame, reset)
   time.Sleep(60 * time.Millisecond)
  }
  fmt.Fprint(w, "\r\x1b[2K")
 }
 fmt.Fprintf(w, "%sOMNI  ·  EVOLUTION%s\n", gold, reset)
 fmt.Fprintln(w, strings.Repeat("─", 32))
}
