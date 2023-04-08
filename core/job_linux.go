package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func Job(args ...any) error {
	projectName := args[0].(string)
	project := args[1].(Project)

	ResultMap.Mu.Lock()
	ResultMap.Map[projectName] = JobState{
		LastBuildStart: time.Now().UTC(),
		StepResults:    make([]Result, 0),
		BuildStatus:    PENDING,
	}
	ResultMap.Mu.Unlock()

	// draining the live result channel whenever a new build starts
	for len(LiveResultUpdates[projectName]) > 0 {
		<-LiveResultUpdates[projectName]
	}

	for _, step := range project.Steps {

		if len(step) == 0 {
			fmt.Fprintln(os.Stderr, "empty step")
			continue
		}

		command := step[0]
		var args []string
		if len(step) > 1 {
			args = step[1:]
		}

		//DONE: create context with deadline from global context
		duration := 10 * time.Minute
		if project.StepTimeout != 0 {
			duration = time.Duration(project.StepTimeout) * time.Second
		}
		ctx, cancel := context.WithTimeout(Ctx, duration)
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Dir = project.Cwd
		// to ensure the sigint sigterm does not get passed to child processes,
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		out, err := cmd.Output()
		cancel()

		//reporting results
		ResultMap.Mu.RLock()
		project, ok := ResultMap.Map[projectName]
		ResultMap.Mu.RUnlock()
		if ok {
			buildTime := project.LastBuildStart
			steps := project.StepResults
			status := project.BuildStatus
			description := "step ran successfully"
			if err != nil {
				description = err.Error()
			}
			steps = append(steps, Result{
				Error:       err,
				Output:      string(out),
				Description: description,
			})

			ResultMap.Mu.Lock()
			ResultMap.Map[projectName] = JobState{
				LastBuildStart: buildTime,
				StepResults:    steps,
				BuildStatus:    status,
			}
			ResultMap.Mu.Unlock()

			// updating the live status
			LiveResultUpdates[projectName] <- JobState{
				LastBuildStart: buildTime,
				StepResults:    steps,
				BuildStatus:    status,
			}

		} else {
			ResultMap.Mu.RUnlock()
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			ResultMap.Mu.RLock()
			obj := ResultMap.Map[projectName]
			ResultMap.Mu.RUnlock()
			obj.BuildStatus = FAILED
			ResultMap.Mu.Lock()
			ResultMap.Map[projectName] = obj
			ResultMap.Mu.Unlock()

			// updating the live status
			LiveResultUpdates[projectName] <- obj
			break
		}
		// } else {
		// 	// LOG: log here when some leveled logger is integrated
		// 	// fmt.Printf("step %v done\n", step)
		// }
	}
	ResultMap.Mu.RLock()
	obj := ResultMap.Map[projectName]
	ResultMap.Mu.RUnlock()
	if obj.BuildStatus != FAILED {
		obj.BuildStatus = SUCCESS
		ResultMap.Mu.Lock()
		ResultMap.Map[projectName] = obj
		ResultMap.Mu.Unlock()

		// updating the live status
		LiveResultUpdates[projectName] <- obj

	}
	return nil
}
