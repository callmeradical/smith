import { refreshLoops } from "./pods.js";
import { refreshDocuments } from "./docs.js";
import { renderProviderList } from "./providers.js";
import { renderProjects, loadProjectsFromStorage } from "./projects.js";
import { state } from "./state.js";

export async function initApp() {
  console.log('INIT: Starting app...');
  state.projects = loadProjectsFromStorage();
  renderProjects();
  renderProviderList();
  
  console.log('INIT: Refreshing loops and documents...');
  void refreshLoops();
  void refreshDocuments();

  // Auto-refresh logic
  setInterval(() => {
    const autoRefreshEl = document.getElementById("auto-refresh");
    if (autoRefreshEl && autoRefreshEl.checked) {
      void refreshLoops();
      void refreshDocuments();
    }
  }, 5000);
}
