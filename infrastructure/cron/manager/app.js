function loadJobs() {
  fetch("/api/jobs")
    .then((r) => r.json())
    .then((data) => {
      // Display environment variables
      const envDiv = document.getElementById("envVars");
      if (data.env && Object.keys(data.env).length > 0) {
        let envHtml =
          "<table><thead><tr><th>Variable</th><th>Value</th></tr></thead><tbody>";
        for (const [key, value] of Object.entries(data.env)) {
          envHtml +=
            "<tr><td>" +
            escapeHtml(key) +
            '</td><td class="command">' +
            escapeHtml(value) +
            "</td></tr>";
        }
        envHtml += "</tbody></table>";
        envDiv.innerHTML = envHtml;
      } else {
        envDiv.innerHTML = "<p>No environment variables found</p>";
      }

      // Update job dropdown
      const jobSelect = document.getElementById("jobSelect");
      // Preserve current selection
      const currentSelection = jobSelect.value;
      // Keep "Select a job..." and "ALL" options, then add jobs
      while (jobSelect.options.length > 2) {
        jobSelect.remove(2);
      }
      const jobs = data.jobs || [];
      jobs.forEach((job) => {
        const option = document.createElement("option");
        option.value = job.id;
        option.textContent = job.id;
        jobSelect.appendChild(option);
      });
      // Restore previous selection if it still exists
      if (
        currentSelection &&
        Array.from(jobSelect.options).some(
          (opt) => opt.value === currentSelection
        )
      ) {
        jobSelect.value = currentSelection;
      }

      // Ensure event listeners are attached after dropdown is populated
      if (!jobSelect.hasAttribute("data-listener-attached")) {
        jobSelect.addEventListener("change", showLogs);
        jobSelect.setAttribute("data-listener-attached", "true");
      }

      const logTypeSelect = document.getElementById("logType");
      if (
        logTypeSelect &&
        !logTypeSelect.hasAttribute("data-listener-attached")
      ) {
        logTypeSelect.addEventListener("change", function () {
          if (jobSelect && jobSelect.value) {
            showLogs();
          }
        });
        logTypeSelect.setAttribute("data-listener-attached", "true");
      }

      // Display jobs
      const tbody = document.getElementById("jobsBody");
      tbody.innerHTML = "";
      jobs.forEach((job) => {
        const row = tbody.insertRow();
        row.insertCell(0).textContent = job.id;

        // Trigger button in column 1
        const triggerCell = row.insertCell(1);
        const triggerBtn = document.createElement("button");
        triggerBtn.textContent = "Trigger";
        triggerBtn.onclick = () => triggerJob(job.id);
        triggerCell.appendChild(triggerBtn);

        // Display all schedules, one per line
        const scheduleCell = row.insertCell(2);
        if (job.schedules && job.schedules.length > 0) {
          scheduleCell.innerHTML = job.schedules
            .map((s) => '<span class="schedule">' + escapeHtml(s) + "</span>")
            .join("<br>");
        } else {
          // Fallback for old format (single schedule)
          scheduleCell.innerHTML =
            '<span class="schedule">' +
            escapeHtml(job.schedule || "") +
            "</span>";
        }
        row.insertCell(3).innerHTML =
          '<span class="command">' + escapeHtml(job.command) + "</span>";
        row.insertCell(4).innerHTML =
          '<span class="comment">' + escapeHtml(job.comment || "") + "</span>";
      });
      document.getElementById("status").textContent =
        "Loaded " + jobs.length + " jobs";
    })
    .catch((err) => {
      document.getElementById("status").textContent =
        "Error loading jobs: " + err;
    });
}

function showLogs() {
  const jobSelect = document.getElementById("jobSelect");
  const selectedJobId = jobSelect.value;

  if (!selectedJobId || selectedJobId === "") {
    document.getElementById("logViewer").textContent =
      "Please select a job first.";
    document.getElementById("logViewer").className = "log-viewer empty";
    return;
  }

  const logType = document.getElementById("logType").value;

  if (selectedJobId === "ALL") {
    // Handle ALL option - fetch logs for all jobs
    fetch("/api/jobs")
      .then((r) => r.json())
      .then((data) => {
        const jobs = data.jobs || [];
        if (jobs.length === 0) {
          document.getElementById("logViewer").textContent = "No jobs found.";
          document.getElementById("logViewer").className = "log-viewer empty";
          return;
        }

        // Fetch logs for all jobs
        const logPromises = jobs.map((job) =>
          fetch(`/api/jobs/${job.id}/logs?type=${logType}`)
            .then((r) => {
              if (r.status === 404) {
                return { id: job.id, content: "No log file found." };
              }
              return logType === "both"
                ? r.json().then((data) => ({
                    id: job.id,
                    stdout: data.stdout || "",
                    stderr: data.stderr || "",
                  }))
                : r.text().then((text) => ({ id: job.id, content: text }));
            })
            .catch((err) => ({
              id: job.id,
              content: "Error loading logs: " + err,
            }))
        );

        Promise.all(logPromises).then((results) => {
          const logViewer = document.getElementById("logViewer");
          let combinedHtml = "";

          results.forEach((result) => {
            combinedHtml += `<div style="margin-top: 20px; padding: 10px; border-top: 2px solid #3e3e42;"><strong>Job: ${escapeHtml(
              result.id
            )}</strong></div>`;

            if (logType === "both") {
              if (result.stdout && result.stdout.trim()) {
                const stdoutLines = result.stdout.split("\n");
                stdoutLines.forEach((line) => {
                  combinedHtml +=
                    '<div class="stdout-line">' + escapeHtml(line) + "</div>";
                });
              }
              if (result.stderr && result.stderr.trim()) {
                const stderrLines = result.stderr.split("\n");
                stderrLines.forEach((line) => {
                  combinedHtml +=
                    '<div class="stderr-line">' + escapeHtml(line) + "</div>";
                });
              }
            } else {
              const content = result.content || "";
              if (content.trim()) {
                const lines = content.split("\n");
                lines.forEach((line) => {
                  const className =
                    logType === "stderr" || logType === "err"
                      ? "stderr-line"
                      : "stdout-line";
                  combinedHtml +=
                    '<div class="' +
                    className +
                    '">' +
                    escapeHtml(line) +
                    "</div>";
                });
              }
            }
          });

          if (!combinedHtml) {
            logViewer.textContent = "No log content found.";
            logViewer.className = "log-viewer empty";
          } else {
            logViewer.innerHTML = combinedHtml;
            logViewer.className = "log-viewer";
            logViewer.scrollTop = logViewer.scrollHeight;
          }
          logViewer.scrollIntoView({
            behavior: "smooth",
            block: "start",
          });
        });
      })
      .catch((err) => {
        const logViewer = document.getElementById("logViewer");
        logViewer.textContent = "Error loading jobs: " + err;
        logViewer.className = "log-viewer empty";
      });
    return;
  }

  // Single job log viewing
  fetch("/api/jobs/" + selectedJobId + "/logs?type=" + logType)
    .then((r) => {
      if (r.status === 404) {
        return r.text().then((text) => {
          throw new Error(text);
        });
      }
      return logType === "both" ? r.json() : r.text();
    })
    .then((data) => {
      const logViewer = document.getElementById("logViewer");

      if (logType === "both") {
        // Handle merged format - array of {source, content} objects
        let combinedHtml = "";

        if (data.merged && Array.isArray(data.merged)) {
          data.merged.forEach((line) => {
            const className =
              line.source === "stderr" ? "stderr-line" : "stdout-line";
            combinedHtml +=
              '<div class="' +
              className +
              '">' +
              escapeHtml(line.content) +
              "</div>";
          });
        } else {
          // Fallback to old format (separate stdout/stderr)
          if (data.stdout && data.stdout.trim()) {
            const stdoutLines = data.stdout.split("\n");
            stdoutLines.forEach((line) => {
              combinedHtml +=
                '<div class="stdout-line">' + escapeHtml(line) + "</div>";
            });
          }

          if (data.stderr && data.stderr.trim()) {
            const stderrLines = data.stderr.split("\n");
            stderrLines.forEach((line) => {
              combinedHtml +=
                '<div class="stderr-line">' + escapeHtml(line) + "</div>";
            });
          }
        }

        if (!combinedHtml) {
          logViewer.textContent = "No log content found.";
          logViewer.className = "log-viewer empty";
        } else {
          logViewer.innerHTML = combinedHtml;
          logViewer.className = "log-viewer";
          logViewer.scrollTop = logViewer.scrollHeight;
        }
        // Scroll to logs section after content is loaded
        logViewer.scrollIntoView({
          behavior: "smooth",
          block: "start",
        });
      } else {
        // Single log (stdout or stderr)
        if (!data || data.trim() === "") {
          logViewer.textContent = "Log file is empty.";
          logViewer.className = "log-viewer empty";
        } else {
          logViewer.textContent = data;
          logViewer.className =
            logType === "stderr" || logType === "err"
              ? "log-viewer stderr-line"
              : "log-viewer";
          logViewer.scrollTop = logViewer.scrollHeight;
        }
        // Scroll to logs section after content is loaded
        logViewer.scrollIntoView({ behavior: "smooth", block: "start" });
      }
    })
    .catch((err) => {
      const logViewer = document.getElementById("logViewer");
      logViewer.textContent = err.message || "Error loading logs: " + err;
      logViewer.className = "log-viewer empty";
    });
}

let currentJobId = null; // Track currently running job
let hasError = false; // Track if current job has errors

function triggerJob(jobId) {
  const triggerStatus = document.getElementById("triggerStatus");
  const triggerLogs = document.getElementById("triggerLogs");
  const killButton = document.getElementById("killButton");
  const retryButton = document.getElementById("retryButton");
  const triggerCheckmark = document.getElementById("triggerCheckmark");
  const triggerSpinner = document.getElementById("triggerSpinner");
  const triggerError = document.getElementById("triggerError");

  currentJobId = jobId;
  hasError = false; // Reset error state
  triggerStatus.textContent = `Triggering job: ${jobId}...`;
  triggerStatus.style.color = "#9cdcfe";
  triggerLogs.innerHTML = "";
  triggerLogs.className = "log-viewer";
  killButton.style.display = "inline-block";

  // Hide retry button, checkmark, and error when starting new job
  // Show spinner while job is running
  retryButton.style.display = "none";
  retryButton.setAttribute("data-job-id", jobId);
  triggerCheckmark.style.display = "none";
  triggerError.style.display = "none";
  triggerSpinner.style.display = "inline-block";

  // Scroll to trigger logs section
  triggerLogs.scrollIntoView({ behavior: "smooth", block: "start" });

  // Use fetch with ReadableStream for POST + SSE streaming
  fetch(`/api/jobs/${jobId}/trigger`, {
    method: "POST",
    headers: {
      Accept: "text/event-stream",
    },
  })
    .then((response) => {
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      function readStream() {
        reader
          .read()
          .then(({ done, value }) => {
            if (done) {
              // Hide spinner
              const triggerSpinner = document.getElementById("triggerSpinner");
              triggerSpinner.style.display = "none";

              const triggerCheckmark =
                document.getElementById("triggerCheckmark");
              const triggerError = document.getElementById("triggerError");

              // Show checkmark or error based on error state
              if (hasError) {
                triggerStatus.textContent = `Job ${jobId} failed`;
                triggerStatus.style.color = "#f48771";
                triggerError.style.display = "inline";
                triggerCheckmark.style.display = "none";
              } else {
                triggerStatus.textContent = `Job ${jobId} completed`;
                triggerStatus.style.color = "#6a9955";
                triggerCheckmark.style.display = "inline";
                triggerError.style.display = "none";
              }

              // Show retry button
              const retryButton = document.getElementById("retryButton");
              retryButton.style.display = "inline-block";

              // Optionally reload logs after completion
              const jobSelect = document.getElementById("jobSelect");
              if (jobSelect && jobSelect.value === jobId) {
                setTimeout(showLogs, 500);
              }
              return;
            }

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split("\n");
            buffer = lines.pop() || ""; // Keep incomplete line in buffer

            let eventType = "message";
            for (const line of lines) {
              if (line.startsWith("event: ")) {
                eventType = line.substring(7).trim();
              } else if (line.startsWith("data: ")) {
                const data = line.substring(6);

                switch (eventType) {
                  case "start":
                    triggerStatus.textContent = data;
                    triggerStatus.style.color = "#9cdcfe";
                    break;
                  case "command":
                    triggerLogs.innerHTML +=
                      '<div class="comment">' + escapeHtml(data) + "</div>";
                    triggerLogs.scrollTop = triggerLogs.scrollHeight;
                    break;
                  case "stdout":
                    triggerLogs.innerHTML +=
                      '<div class="stdout-line">' + escapeHtml(data) + "</div>";
                    triggerLogs.scrollTop = triggerLogs.scrollHeight;
                    break;
                  case "stderr":
                    triggerLogs.innerHTML +=
                      '<div class="stderr-line">' + escapeHtml(data) + "</div>";
                    triggerLogs.scrollTop = triggerLogs.scrollHeight;
                    break;
                  case "error":
                    hasError = true; // Mark that there was an error
                    triggerLogs.innerHTML +=
                      '<div class="stderr-line">ERROR: ' +
                      escapeHtml(data) +
                      "</div>";
                    triggerLogs.scrollTop = triggerLogs.scrollHeight;
                    break;
                  case "complete":
                    triggerStatus.textContent = data;
                    triggerStatus.style.color = "#6a9955";
                    break;
                  case "end":
                    // Hide spinner
                    const triggerSpinner =
                      document.getElementById("triggerSpinner");
                    triggerSpinner.style.display = "none";

                    const triggerCheckmark =
                      document.getElementById("triggerCheckmark");
                    const triggerError =
                      document.getElementById("triggerError");

                    // Show checkmark or error based on error state
                    if (hasError) {
                      triggerStatus.textContent = `Job ${jobId} failed`;
                      triggerStatus.style.color = "#f48771";
                      triggerError.style.display = "inline";
                      triggerCheckmark.style.display = "none";
                    } else {
                      triggerStatus.textContent = `Job ${jobId} completed`;
                      triggerStatus.style.color = "#6a9955";
                      triggerCheckmark.style.display = "inline";
                      triggerError.style.display = "none";
                    }

                    killButton.style.display = "none";
                    currentJobId = null;

                    // Show retry button
                    const retryButton = document.getElementById("retryButton");
                    retryButton.style.display = "inline-block";

                    // Optionally reload logs after completion
                    const jobSelect = document.getElementById("jobSelect");
                    if (jobSelect && jobSelect.value === jobId) {
                      setTimeout(showLogs, 500);
                    }
                    break;
                  default:
                    // Default message handler
                    triggerLogs.innerHTML +=
                      '<div class="stdout-line">' + escapeHtml(data) + "</div>";
                    triggerLogs.scrollTop = triggerLogs.scrollHeight;
                }
              }
            }

            readStream();
          })
          .catch((err) => {
            hasError = true; // Mark that there was an error
            triggerStatus.textContent = `Error streaming job output`;
            triggerStatus.style.color = "#f48771";
            killButton.style.display = "none";
            currentJobId = null;

            // Hide spinner
            const triggerSpinner = document.getElementById("triggerSpinner");
            triggerSpinner.style.display = "none";

            // Show error indicator
            const triggerError = document.getElementById("triggerError");
            const triggerCheckmark =
              document.getElementById("triggerCheckmark");
            triggerError.style.display = "inline";
            triggerCheckmark.style.display = "none";

            triggerLogs.innerHTML +=
              '<div class="stderr-line">Connection error: ' +
              escapeHtml(err.message) +
              "</div>";

            // Show retry button
            const retryButton = document.getElementById("retryButton");
            retryButton.style.display = "inline-block";
          });
      }

      readStream();
    })
    .catch((err) => {
      hasError = true; // Mark that there was an error
      triggerStatus.textContent = `Error triggering job`;
      triggerStatus.style.color = "#f48771";
      killButton.style.display = "none";
      currentJobId = null;

      // Hide spinner
      const triggerSpinner = document.getElementById("triggerSpinner");
      triggerSpinner.style.display = "none";

      // Show error indicator
      const triggerError = document.getElementById("triggerError");
      const triggerCheckmark = document.getElementById("triggerCheckmark");
      triggerError.style.display = "inline";
      triggerCheckmark.style.display = "none";

      triggerLogs.innerHTML +=
        '<div class="stderr-line">Error: ' + escapeHtml(err.message) + "</div>";

      // Show retry button
      const retryButton = document.getElementById("retryButton");
      retryButton.setAttribute("data-job-id", jobId);
      retryButton.style.display = "inline-block";
    });
}

function killJob() {
  if (!currentJobId) {
    return;
  }

  // Save job ID before it might become null
  const jobIdToKill = currentJobId;

  fetch(`/api/jobs/${jobIdToKill}/kill`, { method: "POST" })
    .then((r) => r.json())
    .then((data) => {
      hasError = true; // Mark that there was an error (job was killed)
      const triggerStatus = document.getElementById("triggerStatus");
      // Use job_id from API response, fallback to saved ID
      const killedJobId = data.job_id || jobIdToKill;
      triggerStatus.textContent = `Job ${killedJobId} killed`;
      triggerStatus.style.color = "#f48771";
      document.getElementById("killButton").style.display = "none";

      // Hide spinner
      const triggerSpinner = document.getElementById("triggerSpinner");
      triggerSpinner.style.display = "none";

      // Show error indicator
      const triggerError = document.getElementById("triggerError");
      const triggerCheckmark = document.getElementById("triggerCheckmark");
      triggerError.style.display = "inline";
      triggerCheckmark.style.display = "none";

      // Show retry button
      const retryButton = document.getElementById("retryButton");
      retryButton.setAttribute("data-job-id", killedJobId);
      retryButton.style.display = "inline-block";

      currentJobId = null;
    })
    .catch((err) => {
      alert("Error killing job: " + err);
    });
}

function retryJob() {
  const retryButton = document.getElementById("retryButton");
  const jobId = retryButton.getAttribute("data-job-id") || currentJobId;

  // If we don't have a job ID, try to get it from the status
  if (!jobId) {
    const triggerStatus = document.getElementById("triggerStatus");
    const statusText = triggerStatus.textContent;
    // Try to extract job ID from status (e.g., "Job cheats-detection completed")
    const match = statusText.match(/Job\s+(\S+)/);
    if (match && match[1]) {
      triggerJob(match[1]);
      return;
    }
    alert("Unable to determine job ID to retry");
    return;
  }

  triggerJob(jobId);
}

function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

// Load jobs on page load
loadJobs();

// Setup auto-load logs when dropdowns change
// Use a small delay to ensure elements exist
setTimeout(function () {
  const jobSelect = document.getElementById("jobSelect");
  const logTypeSelect = document.getElementById("logType");

  if (jobSelect) {
    jobSelect.addEventListener("change", function () {
      showLogs();
    });
  }
  if (logTypeSelect) {
    logTypeSelect.addEventListener("change", function () {
      if (jobSelect && jobSelect.value) {
        showLogs();
      }
    });
  }
}, 500);

// Reload every 5 seconds
setInterval(loadJobs, 5000);
