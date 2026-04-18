package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed skills/*.md
var skillsFS embed.FS

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List and print Claude Code skill definitions",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available skills",
	RunE:  runSkillsList,
}

var skillsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print a skill's SKILL.md content",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsShow,
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsShowCmd)
}

func availableSkills() ([]string, error) {
	entries, err := fs.ReadDir(skillsFS, "skills")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			name := strings.TrimSuffix(e.Name(), ".md")
			names = append(names, name)
		}
	}
	return names, nil
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	names, err := availableSkills()
	if err != nil {
		return err
	}
	for _, n := range names {
		fmt.Fprintln(cmd.OutOrStdout(), n)
	}
	return nil
}

func runSkillsShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	path := filepath.Join("skills", name+".md")
	data, err := skillsFS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("skill %q not found (run 'lunar skills list' to see available skills)", name)
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), string(data))
	return err
}
