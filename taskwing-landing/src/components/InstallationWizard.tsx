import { useState, useEffect } from "react";
import "./InstallationWizard.css";

interface InstallMethod {
  id: string;
  name: string;
  icon: string;
  description: string;
  commands: string[];
  requirements?: string[];
}

interface OSInfo {
  name: string;
  platform: string;
  icon: string;
}

export function InstallationWizard() {
  const [currentStep, setCurrentStep] = useState(0);
  const [selectedMethod, setSelectedMethod] = useState("go-install");
  const [detectedOS, setDetectedOS] = useState<OSInfo>({
    name: "Unknown",
    platform: "unknown",
    icon: "💻",
  });
  const [copiedCommand, setCopiedCommand] = useState<string | null>(null);

  // Detect user's operating system
  useEffect(() => {
    const userAgent = navigator.userAgent.toLowerCase();
    const platform = navigator.platform.toLowerCase();

    let os: OSInfo = { name: "Unknown", platform: "unknown", icon: "💻" };

    if (platform.includes("mac") || userAgent.includes("mac")) {
      os = { name: "macOS", platform: "macos", icon: "🍎" };
    } else if (platform.includes("win") || userAgent.includes("windows")) {
      os = { name: "Windows", platform: "windows", icon: "🪟" };
    } else if (platform.includes("linux") || userAgent.includes("linux")) {
      os = { name: "Linux", platform: "linux", icon: "🐧" };
    }

    setDetectedOS(os);
  }, []);

  const installMethods: InstallMethod[] = [
    {
      id: "go-install",
      name: "Go Install",
      icon: "🚀",
      description: "Install directly using Go (recommended)",
      commands: ["go install github.com/josephgoksu/TaskWing@latest"],
      requirements: ["Go 1.19 or later installed"],
    },
    {
      id: "homebrew",
      name: "Homebrew",
      icon: "🍺",
      description: "Install via Homebrew package manager (macOS/Linux)",
      commands: ["brew tap taskwing/taskwing", "brew install taskwing"],
      requirements: ["Homebrew installed"],
    },
    {
      id: "binary",
      name: "Binary Download",
      icon: "📦",
      description: "Download pre-built binary for your platform",
      commands: getOSSpecificBinaryCommands(detectedOS.platform),
      requirements: ["No additional dependencies"],
    },
    {
      id: "source",
      name: "Build from Source",
      icon: "🔧",
      description: "Clone and build from source code",
      commands: [
        "git clone https://github.com/josephgoksu/TaskWing.git",
        "cd taskwing",
        "go build -o taskwing main.go",
        "sudo mv taskwing /usr/local/bin/",
      ],
      requirements: ["Git and Go installed"],
    },
  ];

  function getOSSpecificBinaryCommands(platform: string): string[] {
    switch (platform) {
      case "macos":
        return [
          "curl -L https://github.com/josephgoksu/TaskWing/releases/latest/download/taskwing-darwin-amd64.tar.gz | tar xz",
          "sudo mv taskwing /usr/local/bin/",
        ];
      case "linux":
        return [
          "curl -L https://github.com/josephgoksu/TaskWing/releases/latest/download/taskwing-linux-amd64.tar.gz | tar xz",
          "sudo mv taskwing /usr/local/bin/",
        ];
      case "windows":
        return [
          'Invoke-WebRequest -Uri "https://github.com/josephgoksu/TaskWing/releases/latest/download/taskwing-windows-amd64.zip" -OutFile "taskwing.zip"',
          'Expand-Archive -Path "taskwing.zip" -DestinationPath "."',
          'Move-Item "taskwing.exe" "$env:USERPROFILE\\AppData\\Local\\Microsoft\\WindowsApps\\"',
        ];
      default:
        return [
          "# Download the appropriate binary for your platform from GitHub releases",
        ];
    }
  }

  const steps = [
    {
      title: "Choose Installation Method",
      description: "Select your preferred way to install TaskWing",
      icon: "🎯",
    },
    {
      title: "Install TaskWing",
      description: "Run the installation commands",
      icon: "⚡",
    },
    {
      title: "Initialize Project",
      description: "Set up TaskWing in your project",
      icon: "🏗️",
    },
    {
      title: "Start Using TaskWing",
      description: "Create your first task and explore features",
      icon: "🎉",
    },
  ];

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedCommand(text);
      setTimeout(() => setCopiedCommand(null), 2000);
    } catch (error) {
      console.error("Failed to copy to clipboard:", error);
    }
  };

  const getCurrentMethod = () =>
    installMethods.find((method) => method.id === selectedMethod) ||
    installMethods[0];

  const nextStep = () => {
    if (currentStep < steps.length - 1) {
      setCurrentStep(currentStep + 1);
    }
  };

  const prevStep = () => {
    if (currentStep > 0) {
      setCurrentStep(currentStep - 1);
    }
  };

  return (
    <div className="installation-wizard">
      <div className="wizard-header">
        <h2>Installation Wizard</h2>
        <div className="os-detection">
          <span className="os-icon">{detectedOS.icon}</span>
          <span className="os-name">Detected: {detectedOS.name}</span>
        </div>
      </div>

      {/* Progress Steps */}
      <div className="progress-steps">
        {steps.map((step, index) => (
          <div
            key={index}
            className={`progress-step ${index <= currentStep ? "active" : ""} ${
              index === currentStep ? "current" : ""
            }`}
          >
            <div className="step-indicator">
              <span className="step-icon">{step.icon}</span>
              <span className="step-number">{index + 1}</span>
            </div>
            <div className="step-content">
              <div className="step-title">{step.title}</div>
              <div className="step-description">{step.description}</div>
            </div>
          </div>
        ))}
      </div>

      {/* Wizard Content */}
      <div className="wizard-content">
        {currentStep === 0 && (
          <div className="method-selection">
            <h3>Choose Your Installation Method</h3>
            <div className="method-tabs">
              {installMethods.map((method) => (
                <button
                  key={method.id}
                  className={`method-tab ${
                    selectedMethod === method.id ? "active" : ""
                  }`}
                  onClick={() => setSelectedMethod(method.id)}
                >
                  <span className="method-icon">{method.icon}</span>
                  <span className="method-name">{method.name}</span>
                </button>
              ))}
            </div>

            <div className="method-details">
              <div className="method-info">
                <h4>{getCurrentMethod().name}</h4>
                <p>{getCurrentMethod().description}</p>
                {getCurrentMethod().requirements && (
                  <div className="requirements">
                    <strong>Requirements:</strong>
                    <ul>
                      {getCurrentMethod().requirements?.map((req, index) => (
                        <li key={index}>{req}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {currentStep === 1 && (
          <div className="installation-commands">
            <h3>Install TaskWing</h3>
            <div className="selected-method">
              <div className="method-header">
                <span className="method-icon">{getCurrentMethod().icon}</span>
                <span className="method-name">{getCurrentMethod().name}</span>
              </div>

              <div className="commands-list">
                {getCurrentMethod().commands.map((command, index) => (
                  <div key={index} className="command-block">
                    <div className="command-header">
                      <span className="command-step-label">Step {index + 1}</span>
                      <button
                        className={`copy-button ${
                          copiedCommand === command ? "copied" : ""
                        }`}
                        onClick={() => copyToClipboard(command)}
                      >
                        {copiedCommand === command ? "✓ Copied!" : "📋 Copy"}
                      </button>
                    </div>
                    <div className="command-text">
                      <code>{command}</code>
                    </div>
                  </div>
                ))}
              </div>

              <div className="verification">
                <h4>Verify Installation</h4>
                <div className="command-block">
                  <div className="command-header">
                    <span className="command-step-label">Verify</span>
                    <button
                      className={`copy-button ${
                        copiedCommand === "taskwing --version" ? "copied" : ""
                      }`}
                      onClick={() => copyToClipboard("taskwing --version")}
                    >
                      {copiedCommand === "taskwing --version"
                        ? "✓ Copied!"
                        : "📋 Copy"}
                    </button>
                  </div>
                  <div className="command-text">
                    <code>taskwing --version</code>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {currentStep === 2 && (
          <div className="project-setup">
            <h3>Initialize Your Project</h3>
            <p>Set up TaskWing in your current project directory:</p>

            <div className="command-block">
              <div className="command-header">
                <span className="command-step-label">Initialize</span>
                <button
                  className={`copy-button ${
                    copiedCommand === "taskwing init" ? "copied" : ""
                  }`}
                  onClick={() => copyToClipboard("taskwing init")}
                >
                  {copiedCommand === "taskwing init" ? "✓ Copied!" : "📋 Copy"}
                </button>
              </div>
              <div className="command-text">
                <code>taskwing init</code>
              </div>
            </div>

            <div className="setup-explanation">
              <h4>What this does:</h4>
              <ul>
                <li>
                  Creates a <code>.taskwing</code> directory in your project
                </li>
                <li>Sets up the default configuration file</li>
                <li>Initializes the tasks database</li>
                <li>Configures project-specific settings</li>
              </ul>
            </div>
          </div>
        )}

        {currentStep === 3 && (
          <div className="getting-started">
            <h3>Start Using TaskWing</h3>
            <p>You're all set! Try these commands to get started:</p>

            <div className="starter-commands">
              <div className="command-block">
                <div className="command-header">
                  <span className="command-step-label">Create Task</span>
                  <button
                    className={`copy-button ${
                      copiedCommand === 'taskwing add "My first task"'
                        ? "copied"
                        : ""
                    }`}
                    onClick={() =>
                      copyToClipboard('taskwing add "My first task"')
                    }
                  >
                    {copiedCommand === 'taskwing add "My first task"'
                      ? "✓ Copied!"
                      : "📋 Copy"}
                  </button>
                </div>
                <div className="command-text">
                  <code>taskwing add "My first task"</code>
                </div>
              </div>

              <div className="command-block">
                <div className="command-header">
                  <span className="command-step-label">List Tasks</span>
                  <button
                    className={`copy-button ${
                      copiedCommand === "taskwing list" ? "copied" : ""
                    }`}
                    onClick={() => copyToClipboard("taskwing list")}
                  >
                    {copiedCommand === "taskwing list"
                      ? "✓ Copied!"
                      : "📋 Copy"}
                  </button>
                </div>
                <div className="command-text">
                  <code>taskwing list</code>
                </div>
              </div>

              <div className="command-block">
                <div className="command-header">
                  <span className="command-step-label">Start MCP Server</span>
                  <button
                    className={`copy-button ${
                      copiedCommand === "taskwing mcp" ? "copied" : ""
                    }`}
                    onClick={() => copyToClipboard("taskwing mcp")}
                  >
                    {copiedCommand === "taskwing mcp" ? "✓ Copied!" : "📋 Copy"}
                  </button>
                </div>
                <div className="command-text">
                  <code>taskwing mcp</code>
                </div>
              </div>
            </div>

            <div className="next-steps">
              <h4>Next Steps:</h4>
              <ul>
                <li>
                  <a
                    href="https://github.com/josephgoksu/TaskWing#documentation"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    📚 Read the full documentation
                  </a>
                </li>
                <li>
                  <a
                    href="https://github.com/josephgoksu/TaskWing/blob/main/examples"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    💡 Explore usage examples
                  </a>
                </li>
                <li>
                  <a
                    href="https://github.com/josephgoksu/TaskWing/discussions"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    💬 Join the community discussions
                  </a>
                </li>
                <li>
                  <a
                    href="https://github.com/josephgoksu/TaskWing/issues"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    🐛 Report issues or request features
                  </a>
                </li>
              </ul>
            </div>
          </div>
        )}
      </div>

      {/* Navigation */}
      <div className="wizard-navigation">
        <button
          className="nav-button secondary"
          onClick={prevStep}
          disabled={currentStep === 0}
        >
          ← Previous
        </button>

        <div className="step-indicator-dots">
          {steps.map((_, index) => (
            <button
              key={index}
              className={`step-dot ${index <= currentStep ? "active" : ""}`}
              onClick={() => setCurrentStep(index)}
            />
          ))}
        </div>

        <button
          className="nav-button primary"
          onClick={nextStep}
          disabled={currentStep === steps.length - 1}
        >
          {currentStep === steps.length - 1 ? "🎉 Complete!" : "Next →"}
        </button>
      </div>
    </div>
  );
}
