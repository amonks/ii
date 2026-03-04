package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var descriptionFlagAliases = map[string]string{
	"desc": "description",
}

func addDescriptionFlagAliases(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		setFlagAliases(cmd.Flags(), descriptionFlagAliases)
	}
}

func setFlagAliases(flags *pflag.FlagSet, aliases map[string]string) {
	if len(aliases) == 0 {
		return
	}

	normalize := flags.GetNormalizeFunc()
	flags.SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		if alias, ok := aliases[name]; ok {
			name = alias
		}
		return normalize(f, name)
	})
}
