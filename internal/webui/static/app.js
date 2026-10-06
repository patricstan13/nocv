(function () {
  "use strict";

  const inspector = document.getElementById("inspector");
  const analysisStatusIndicator = document.getElementById("analysis-status");
  const networkElement = document.getElementById("network");
  const typeNetworkElement = document.getElementById("type-network");
  const typeContext = document.getElementById("type-context");
  const typeContextTitle = document.getElementById("type-context-title");
  const backToPackages = document.getElementById("back-to-packages");
  const searchInput = document.getElementById("package-search-input");
  const searchResults = document.getElementById("package-search-results");
  const SEARCH_RESULT_LIMIT = 10;
  let packageNetwork;
  let packageEdges;
  let packageSearchControl;
  let typeNetwork;
  let typeFreezeFallback;
  let currentPackageDependency;
  let packageLabels = new Map();
  let packageDependencyOriginRef = null;
  let inspectorRequestGeneration = 0;
  let inspectorState = { kind: "symbol", inspection: null, proposal: null, result: null };

  function element(tag, text, className) {
    const item = document.createElement(tag);
    if (text !== undefined) item.textContent = text;
    if (className) item.className = className;
    return item;
  }

  async function getJSON(url) {
    const response = await fetch(url, { headers: { Accept: "application/json" } });
    let body;
    try {
      body = await response.json();
    } catch (_) {
      throw new Error("The server returned an unreadable response.");
    }
    if (!response.ok) throw new Error(body.error || "Request failed.");
    return body;
  }

  async function postJSON(url, payload) {
    const response = await fetch(url, {
      method: "POST",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    let body;
    try {
      body = await response.json();
    } catch (_) {
      throw new Error("The server returned an unreadable response.");
    }
    if (!response.ok) throw new Error(body.error || "Request failed.");
    return body;
  }

  function replaceInspector(parts) {
    inspector.replaceChildren(...parts);
    inspector.scrollTop = 0;
  }

  function beginInspectorRequest(label) {
    const generation = ++inspectorRequestGeneration;
    replaceInspector([
      element("p", "Inspector", "eyebrow"),
      element("h2", "Loading " + (label || "selection") + "…"),
    ]);
    return generation;
  }

  function invalidateInspectorRequests() {
    inspectorRequestGeneration += 1;
  }

  function section(title, values, render) {
    const parts = [element("h3", title)];
    if (!values || values.length === 0) {
      parts.push(element("p", "None", "muted"));
      return parts;
    }
    const list = element("ul");
    values.forEach((value) => list.appendChild(render(value)));
    parts.push(list);
    return parts;
  }

  function nonEmptySection(title, values, render) {
    if (!values || values.length === 0) return [];
    return section(title, values, render);
  }

  function disclosure(label, parts, className) {
    const details = element("details", undefined, className || "advanced-disclosure");
    details.appendChild(element("summary", label));
    const body = element("div", undefined, "disclosure-body");
    body.append(...parts);
    details.appendChild(body);
    return details;
  }

  function relationshipItem(value) {
    const item = element("li", undefined, "relationship");
    const summary = element("div");
    summary.appendChild(element("strong", value.kind));
    const from = typeof value.from === "string" ? value.from : value.from.ref;
    const to = typeof value.to === "string" ? value.to : value.to.ref;
    summary.appendChild(document.createTextNode("  " + from + " → " + to));
    item.appendChild(summary);
    const evidence = value.evidence || [];
    if (evidence.length > 0) {
      item.appendChild(element("span", evidence.map(formatLocation).join(" · "), "evidence"));
    }
    return item;
  }

  function evidenceDisclosure(evidence) {
    if (!evidence || evidence.length === 0) return null;
    const list = element("ul", undefined, "evidence-list");
    evidence.forEach((location) => list.appendChild(element("li", formatLocation(location))));
    return disclosure("Evidence (" + evidence.length + ")", [list], "evidence-disclosure");
  }

  function symbolDisplayName(summary) {
    if (summary.kind === "function" && summary.parentRef && summary.parentName && !packageLabels.has(summary.parentRef)) {
      return summary.parentName + "." + summary.name;
    }
    return summary.name;
  }

  function renderSymbolButton(summary) {
    const button = element("button", symbolDisplayName(summary), "symbol-button");
    button.type = "button";
    button.title = summary.ref;
    button.setAttribute("aria-label", symbolDisplayName(summary) + " (" + summary.ref + ")");
    button.addEventListener("click", () => inspectSymbol(summary.ref));
    return button;
  }

  function symbolItem(summary) {
    const item = element("li", undefined, "symbol-row");
    item.appendChild(renderSymbolButton(summary));
    return item;
  }

  function packageContentItem(node) {
    return symbolItem({
      ref: node.id,
      kind: node.kind,
      name: node.name,
      parentRef: node.parent,
      parentName: node.parentName,
    });
  }

  function refSymbolItem(ref) {
    const item = element("li", undefined, "symbol-row");
    const button = element("button", typeLabel(ref), "symbol-button");
    button.type = "button";
    button.title = ref;
    button.setAttribute("aria-label", typeLabel(ref) + " (" + ref + ")");
    button.addEventListener("click", () => inspectSymbol(ref));
    item.appendChild(button);
    return item;
  }

  function symbolRelationshipItem(relationship, incoming, showKind) {
    const item = element("li", undefined, "symbol-row relationship-symbol-row");
    const summary = incoming ? relationship.from : relationship.to;
    item.appendChild(renderSymbolButton(summary));
    if (showKind) item.appendChild(element("span", relationship.kind, "relationship-kind"));
    const evidence = evidenceDisclosure(relationship.evidence || []);
    if (evidence) item.appendChild(evidence);
    return item;
  }

  function dependencyTargetItem(value) {
    return element("li", value.to, "symbol");
  }

  function dependencySourceItem(value) {
    return element("li", value.from, "symbol");
  }

  function packageLabel(ref) {
    const components = ref.split("/");
    return packageLabels.get(ref) || components[components.length - 1];
  }

  function packageRelationshipItem(value, ref, originPackageRef) {
    const item = element("li", undefined, "package-relationship");
    const label = packageLabel(ref);
    const button = element("button", label, "package-relationship-button");
    button.type = "button";
    button.title = ref;
    button.setAttribute("aria-label", label + " (" + ref + ")");
    button.addEventListener("click", () => navigateToPackageDependency(value, originPackageRef));
    item.appendChild(button);
    return item;
  }

  function packageDependencyItem(value, originPackageRef) {
    return packageRelationshipItem(value, value.to, originPackageRef);
  }

  function packageDependentItem(value, originPackageRef) {
    return packageRelationshipItem(value, value.from, originPackageRef);
  }

  function packageEndpointItem(ref) {
    const item = element("li", undefined, "package-relationship");
    const label = packageLabel(ref);
    const button = element("button", label, "package-relationship-button");
    button.type = "button";
    button.title = ref;
    button.setAttribute("aria-label", label + " (" + ref + ")");
    button.addEventListener("click", () => selectPackage(ref, true));
    item.appendChild(button);
    return item;
  }

  function collapsibleRelationships(values, threshold, showLabel) {
    const container = element("div", undefined, "evidence-group");
    const list = element("ul");
    values.forEach((value) => list.appendChild(relationshipItem(value)));
    if (values.length <= threshold) {
      container.appendChild(list);
      return container;
    }

    list.hidden = true;
    const toggle = element("button", showLabel, "evidence-toggle");
    toggle.type = "button";
    toggle.setAttribute("aria-expanded", "false");
    toggle.addEventListener("click", () => {
      const expanded = list.hidden;
      list.hidden = !expanded;
      toggle.textContent = expanded ? "Hide evidence" : showLabel;
      toggle.setAttribute("aria-expanded", String(expanded));
    });
    container.append(toggle, list);
    return container;
  }

  function formatLocation(value) {
    if (!value.file) return "unknown location";
    return value.file + (value.line && value.column ? ":" + value.line + ":" + value.column : "");
  }

  function compareText(left, right) {
    if (left < right) return -1;
    if (left > right) return 1;
    return 0;
  }

  function searchPackages(packageNodes, query) {
    const normalized = query.trim().toLowerCase();
    if (!normalized) return { matches: [], total: 0 };

    const ranked = packageNodes.map((node) => {
      const label = node.label.toLowerCase();
      const ref = node.id.toLowerCase();
      let rank = 4;
      if (label === normalized) rank = 0;
      else if (label.startsWith(normalized)) rank = 1;
      else if (label.includes(normalized)) rank = 2;
      else if (ref.includes(normalized)) rank = 3;
      return { node, rank, label, ref };
    }).filter((candidate) => candidate.rank < 4);

    ranked.sort((left, right) => left.rank - right.rank || compareText(left.label, right.label) || compareText(left.ref, right.ref));
    return {
      matches: ranked.slice(0, SEARCH_RESULT_LIMIT).map((candidate) => candidate.node),
      total: ranked.length,
    };
  }

  function setupPackageSearch(packageNodes) {
    let visibleResults = [];
    let activeIndex = -1;

    function closeSearchResults() {
      searchResults.hidden = true;
      searchInput.setAttribute("aria-expanded", "false");
      activeIndex = -1;
    }

    function setActiveIndex(index) {
      if (visibleResults.length === 0) return;
      activeIndex = (index + visibleResults.length) % visibleResults.length;
      Array.from(searchResults.querySelectorAll(".package-search-result")).forEach((item, itemIndex) => {
        const active = itemIndex === activeIndex;
        item.classList.toggle("is-active", active);
        item.setAttribute("aria-selected", String(active));
        if (active) item.scrollIntoView({ block: "nearest" });
      });
    }

    function selectSearchResult(node) {
      selectPackage(node.id, true);
      closeSearchResults();
    }

    function renderSearchResults() {
      const query = searchInput.value;
      if (!query.trim()) {
        visibleResults = [];
        searchResults.replaceChildren();
        closeSearchResults();
        return;
      }

      const result = searchPackages(packageNodes, query);
      visibleResults = result.matches;
      const parts = [];
      visibleResults.forEach((node, index) => {
        const item = element("button", undefined, "package-search-result");
        item.type = "button";
        item.setAttribute("role", "option");
        item.setAttribute("aria-selected", "false");
        item.appendChild(element("span", node.label, "package-search-label"));
        item.appendChild(element("span", node.id, "package-search-ref"));
        item.addEventListener("mouseenter", () => setActiveIndex(index));
        item.addEventListener("click", () => selectSearchResult(node));
        parts.push(item);
      });
      if (result.total === 0) {
        parts.push(element("div", "No matching packages", "package-search-status"));
      } else if (result.total > visibleResults.length) {
        parts.push(element("div", "+ " + (result.total - visibleResults.length) + " more matches", "package-search-status"));
      }
      searchResults.replaceChildren(...parts);
      searchResults.hidden = false;
      searchInput.setAttribute("aria-expanded", "true");
      activeIndex = visibleResults.length > 0 ? 0 : -1;
      if (activeIndex >= 0) setActiveIndex(activeIndex);
    }

    searchInput.addEventListener("input", renderSearchResults);
    searchInput.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeSearchResults();
      } else if (event.key === "ArrowDown" && visibleResults.length > 0) {
        event.preventDefault();
        setActiveIndex(activeIndex + 1);
      } else if (event.key === "ArrowUp" && visibleResults.length > 0) {
        event.preventDefault();
        setActiveIndex(activeIndex - 1);
      } else if (event.key === "Enter" && activeIndex >= 0) {
        event.preventDefault();
        selectSearchResult(visibleResults[activeIndex]);
      }
    });

    return {
      setEnabled(enabled) {
        searchInput.disabled = !enabled;
        if (!enabled) closeSearchResults();
      },
    };
  }

  function renderPackageInspection(result) {
    const detail = result.package;
    const parts = [
      element("p", "Package", "eyebrow"),
      element("h2", result.node.id),
      element("span", result.node.kind, "kind"),
      element("h3", "Documentation"),
      element("p", result.node.documentation || "No package documentation.", result.node.documentation ? "documentation" : "muted"),
    ];
    parts.push(...section("Dependencies", detail.dependencies, (value) => packageDependencyItem(value, result.node.id)));
    parts.push(...section("Dependents", detail.dependents, (value) => packageDependentItem(value, result.node.id)));
    parts.push(...nonEmptySection("Types", detail.types, packageContentItem));
    parts.push(...nonEmptySection("Functions", detail.functions, packageContentItem));

    const advanced = [];
    advanced.push(...nonEmptySection("Go imports (" + (detail.imports || []).length + ")", detail.imports, dependencyTargetItem));
    advanced.push(...nonEmptySection("Imported by (" + (detail.importers || []).length + ")", detail.importers, dependencySourceItem));
    if (advanced.length > 0) parts.push(disclosure("Advanced", advanced));
    replaceInspector(parts);
  }

  function showPackage(id) {
    packageDependencyOriginRef = null;
    inspectSymbol(id);
  }

  function typeLabel(ref) {
    const components = ref.split("::");
    return components[components.length - 1];
  }

  function typeTargetItem(value) {
    return refSymbolItem(value.to);
  }

  function typeSourceItem(value) {
    return refSymbolItem(value.from);
  }

  function renderPackageDependencyInspection(result) {
    const parts = [];
    if (packageDependencyOriginRef && packageLabels.has(packageDependencyOriginRef)) {
      const label = packageLabel(packageDependencyOriginRef);
      const back = element("button", "← Back to " + label, "back-button dependency-back-button");
      back.type = "button";
      back.title = packageDependencyOriginRef;
      back.setAttribute("aria-label", "Back to package " + packageDependencyOriginRef);
      back.addEventListener("click", returnToPackageOrigin);
      parts.push(back);
    }
    const semanticFrom = result.dependency.from;
    const semanticTo = result.dependency.to;
    parts.push(
      element("p", "Semantic dependency", "eyebrow"),
      element("h2", semanticFrom + " depends on " + semanticTo),
    );
    parts.push(...section("From", [semanticFrom], packageEndpointItem));
    parts.push(...section("To", [semanticTo], packageEndpointItem));
    parts.push(...section("Type relationships", result.typeDependencies, (dependency) => {
      const item = element("li", undefined, "relationship");
      const facts = dependency.evidence || [];
      const heading = element("div", undefined, "type-relationship-heading");
      heading.appendChild(element("div", dependency.from + " → " + dependency.to + " (" + facts.length + " facts)", "symbol"));
      const view = element("button", "View type graph", "type-view-button");
      view.type = "button";
      view.addEventListener("click", () => showTypeDrilldown(result));
      heading.appendChild(view);
      item.appendChild(heading);
      item.appendChild(collapsibleRelationships(facts, 2, "Show evidence"));
      return item;
    }));
    const exact = result.exactOnly || [];
    parts.push(element("h3", "Exact-only relationships (" + exact.length + ")"));
    if (exact.length === 0) {
      parts.push(element("p", "None", "muted"));
    } else {
      parts.push(collapsibleRelationships(exact, 5, "Show relationships"));
    }
    replaceInspector(parts);
  }

  async function showDependency(from, to) {
    const generation = beginInspectorRequest(packageLabel(from) + " dependency");
    try {
      const url = "/api/package-dependency?from=" + encodeURIComponent(from) + "&to=" + encodeURIComponent(to);
      const result = await getJSON(url);
      if (generation !== inspectorRequestGeneration) return;
      currentPackageDependency = { from, to, result };
      renderPackageDependencyInspection(result);
    } catch (error) {
      if (generation !== inspectorRequestGeneration) return;
      showError(error);
    }
  }

  function findPackageEdge(relationship) {
    return packageEdges.get().find((edge) => edge.semanticFrom === relationship.from && edge.semanticTo === relationship.to);
  }

  function selectPackage(ref, focusNode) {
    packageNetwork.selectNodes([ref], true);
    if (focusNode) {
      packageNetwork.focus(ref, {
        scale: 0.75,
        animation: { duration: 450, easingFunction: "easeInOutQuad" },
      });
    }
    showPackage(ref);
  }

  function openPackageDependency(from, to, edge, focusEndpoints, originPackageRef) {
    packageDependencyOriginRef = originPackageRef || null;
    if (edge) {
      packageNetwork.setSelection({ edges: [edge.id] }, { unselectAll: true, highlightEdges: false });
      if (focusEndpoints) {
        packageNetwork.fit({
          nodes: [edge.from, edge.to],
          maxZoomLevel: 0.85,
          animation: { duration: 450, easingFunction: "easeInOutQuad" },
        });
      }
    }
    showDependency(from, to);
  }

  function navigateToPackageDependency(relationship, originPackageRef) {
    const edge = findPackageEdge(relationship);
    openPackageDependency(relationship.from, relationship.to, edge, true, originPackageRef);
  }

  function returnToPackageOrigin() {
    const originPackageRef = packageDependencyOriginRef;
    if (!originPackageRef || !packageLabels.has(originPackageRef)) return;
    selectPackage(originPackageRef, true);
  }

  function showTypeDrilldown(result) {
    const dependencies = result.typeDependencies || [];
    if (dependencies.length === 0 || !currentPackageDependency) return;
    invalidateInspectorRequests();

    const byRef = new Map();
    dependencies.forEach((dependency) => {
      byRef.set(dependency.from, { id: dependency.from, label: typeLabel(dependency.from), title: dependency.from, shape: "box", margin: 12 });
      byRef.set(dependency.to, { id: dependency.to, label: typeLabel(dependency.to), title: dependency.to, shape: "box", margin: 12 });
    });
    const typeNodes = new vis.DataSet(Array.from(byRef.values()));
    const typeEdges = new vis.DataSet(dependencies.map((dependency, index) => ({
      id: "type-dependency-" + index,
      from: dependency.to,
      to: dependency.from,
      semanticFrom: dependency.from,
      semanticTo: dependency.to,
      evidence: dependency.evidence || [],
      arrows: { to: { enabled: true, scaleFactor: 0.7 } },
    })));

    networkElement.hidden = true;
    typeNetworkElement.hidden = false;
    typeContext.hidden = false;
    typeContextTitle.textContent = currentPackageDependency.from + " depends on " + currentPackageDependency.to;
    packageSearchControl.setEnabled(false);
    window.clearTimeout(typeFreezeFallback);
    if (typeNetwork) typeNetwork.destroy();
    const activeTypeNetwork = new vis.Network(typeNetworkElement, { nodes: typeNodes, edges: typeEdges }, {
      layout: { randomSeed: 240519, improvedLayout: true },
      physics: {
        enabled: true,
        stabilization: { enabled: true, iterations: 400, fit: true },
        barnesHut: { gravitationalConstant: -3200, springLength: 170, springConstant: 0.035 },
      },
      interaction: { dragNodes: true, dragView: true, hover: true, hoverConnectedEdges: true, multiselect: false, selectConnectedEdges: true, zoomView: true },
      nodes: {
        borderWidth: 1,
        color: { background: "#ffffff", border: "#64748b", highlight: { background: "#cffafe", border: "#0891b2" } },
        font: { color: "#172033", face: "system-ui", size: 14 },
      },
      edges: {
        color: { color: "#38bdf8", highlight: "#0369a1", hover: "#0284c7" },
        width: 1.5,
        selectionWidth: 2,
        smooth: { enabled: true, type: "dynamic" },
      },
    });
    typeNetwork = activeTypeNetwork;

    let typePhysicsFrozen = false;
    const freezeTypePhysics = () => {
      if (typePhysicsFrozen || typeNetwork !== activeTypeNetwork) return;
      typePhysicsFrozen = true;
      window.clearTimeout(typeFreezeFallback);
      activeTypeNetwork.stopSimulation();
      activeTypeNetwork.setOptions({ physics: false });
    };
    activeTypeNetwork.once("stabilizationIterationsDone", freezeTypePhysics);
    activeTypeNetwork.once("stabilized", freezeTypePhysics);
    typeFreezeFallback = window.setTimeout(freezeTypePhysics, 5000);
    activeTypeNetwork.on("click", (params) => {
      if (params.nodes.length > 0) {
        inspectSymbol(params.nodes[0]);
      } else if (params.edges.length > 0) {
        showTypeDependency(typeEdges.get(params.edges[0]));
      }
    });

    replaceInspector([
      element("p", "Type relationships", "eyebrow"),
      element("h2", currentPackageDependency.from + " depends on " + currentPackageDependency.to),
      element("p", dependencies.length + " type relationships explain this package dependency.", "documentation"),
      element("p", "Select a type or type dependency to inspect it.", "muted"),
    ]);
  }

  function renderTypeInspection(result) {
    const detail = result.type;
    const parts = [
      element("p", result.node.kind, "eyebrow"),
      element("h2", result.node.name),
      element("p", result.node.id, "symbol identity-line"),
      element("h3", "Documentation"),
      element("p", result.node.documentation || "No type documentation.", result.node.documentation ? "documentation" : "muted"),
    ];
    parts.push(...nonEmptySection("Methods", detail.methods, symbolItem));
    parts.push(...nonEmptySection("Dependencies", detail.dependencies, typeTargetItem));
    parts.push(...nonEmptySection("Dependents", detail.dependents, typeSourceItem));

    const exactParts = [];
    exactParts.push(...nonEmptySection("Direct dependencies", detail.directDependencies, (value) => symbolRelationshipItem(value, false, true)));
    exactParts.push(...nonEmptySection("Direct dependents", detail.directDependents, (value) => symbolRelationshipItem(value, true, true)));
    if (exactParts.length > 0) parts.push(disclosure("Advanced · Exact relationships", exactParts));
    replaceInspector(parts);
  }

  function callableHeading(result) {
    const method = result.node.parentKind === "struct" || result.node.parentKind === "interface";
    return method ? result.node.parentName + "." + result.node.name : result.node.name;
  }

  function callableKind(result) {
    return result.node.parentKind === "struct" || result.node.parentKind === "interface" ? "method" : "function";
  }

  function formatCallableSignature(name, signature) {
    const parameters = (signature.parameters || []).map((parameter, index) => {
      const prefix = signature.variadic && index === signature.parameters.length - 1 ? "..." : "";
      return prefix + parameter.type.display;
    });
    const results = (signature.results || []).map((result) => result.type.display);
    let resultDisplay = "";
    if (results.length === 1) resultDisplay = " " + results[0];
    if (results.length > 1) resultDisplay = " (" + results.join(", ") + ")";
    return name + "(" + parameters.join(", ") + ")" + resultDisplay;
  }

  function initialSignatureProposal(result) {
    return {
      parameters: result.function.signature.parameters.map((parameter) => parameter.type.display),
      results: (result.function.signature.results || []).map((value) => value.type.display),
      variadic: result.function.signature.variadic,
    };
  }

  function renderWorkflowButton(label, className, action) {
    const button = element("button", label, className || "workflow-button");
    button.type = "button";
    button.addEventListener("click", action);
    return button;
  }

  function beginSignatureChange(result) {
    invalidateInspectorRequests();
    renderSignatureChangeEditor({
	      kind: "signature-edit",
      inspection: result,
      proposal: initialSignatureProposal(result),
      result: null,
    });
  }

  function returnToCallableInspection(inspection) {
    invalidateInspectorRequests();
    renderFunctionInspection(inspection);
  }

  function renderSignatureChangeEditor(state, message) {
    inspectorState = state;
    const heading = callableHeading(state.inspection);
    const signature = state.inspection.function.signature;
    const parts = [
      element("p", "Signature change", "eyebrow"),
      element("h2", heading),
      element("h3", "Current"),
      element("p", formatCallableSignature(heading, signature), "signature-line"),
      element("h3", "Parameters"),
    ];
    if (message) parts.push(element("p", message, "error"));

    const form = element("form", undefined, "parameter-editor");
    state.proposal.parameters.forEach((typeExpression, index) => {
      const row = element("div", undefined, "parameter-row");
      const label = element("label", undefined, "visually-hidden");
      label.htmlFor = "parameter-type-" + index;
      label.textContent = "Parameter " + (index + 1) + " type";
      const input = element("input", undefined, "parameter-input");
      input.id = label.htmlFor;
      input.type = "text";
      input.value = typeExpression;
      input.placeholder = "Go type expression";
      input.addEventListener("input", () => { state.proposal.parameters[index] = input.value; });
      const remove = renderWorkflowButton("×", "parameter-remove", () => {
        state.proposal.parameters.splice(index, 1);
        if (state.proposal.parameters.length === 0) state.proposal.variadic = false;
        renderSignatureChangeEditor(state);
      });
      remove.setAttribute("aria-label", "Remove parameter " + (index + 1));
      row.append(label, input, remove);
      form.appendChild(row);
    });

    const add = renderWorkflowButton("+ Add parameter", "workflow-button secondary", () => {
      state.proposal.parameters.push("");
      renderSignatureChangeEditor(state);
    });
    add.setAttribute("aria-label", "Add parameter");
    form.appendChild(add);

    form.appendChild(element("h3", "Results"));
    state.proposal.results.forEach((typeExpression, index) => {
      const row = element("div", undefined, "parameter-row");
      const label = element("label", undefined, "visually-hidden");
      label.htmlFor = "result-type-" + index;
      label.textContent = "Result " + (index + 1) + " type";
      const input = element("input", undefined, "parameter-input");
      input.id = label.htmlFor;
      input.type = "text";
      input.value = typeExpression;
      input.placeholder = "Go type expression";
      input.addEventListener("input", () => { state.proposal.results[index] = input.value; });
      const remove = renderWorkflowButton("×", "parameter-remove", () => {
        state.proposal.results.splice(index, 1);
        renderSignatureChangeEditor(state);
      });
      remove.setAttribute("aria-label", "Remove result " + (index + 1));
      row.append(label, input, remove);
      form.appendChild(row);
    });
    const addResult = renderWorkflowButton("+ Add result", "workflow-button secondary", () => {
      state.proposal.results.push("");
      renderSignatureChangeEditor(state);
    });
    addResult.setAttribute("aria-label", "Add result");
    form.appendChild(addResult);

    const variadicLabel = element("label", undefined, "variadic-control");
    const variadic = element("input");
    variadic.type = "checkbox";
    variadic.checked = state.proposal.variadic;
    variadic.disabled = state.proposal.parameters.length === 0;
    variadic.setAttribute("aria-label", "Final parameter is variadic");
    variadic.addEventListener("change", () => { state.proposal.variadic = variadic.checked; });
    variadicLabel.append(variadic, document.createTextNode(" Final parameter is variadic"));
    form.appendChild(variadicLabel);

    const actions = element("div", undefined, "workflow-actions");
    actions.append(
      renderWorkflowButton("Analyze", "workflow-button primary", () => requestSignatureImpact(state)),
      renderWorkflowButton("Cancel", "workflow-button secondary", () => returnToCallableInspection(state.inspection)),
    );
    form.appendChild(actions);
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      requestSignatureImpact(state);
    });
    parts.push(form);
    replaceInspector(parts);
  }

  async function requestSignatureImpact(state) {
    const parameters = state.proposal.parameters.map((value) => value.trim());
	    const results = state.proposal.results.map((value) => value.trim());
    if (parameters.some((value) => value === "")) {
      renderSignatureChangeEditor(state, "Every parameter needs a type expression.");
      return;
    }
	    if (results.some((value) => value === "")) {
	      renderSignatureChangeEditor(state, "Every result needs a type expression.");
	      return;
	    }
    if (state.proposal.variadic && parameters.length === 0) {
      renderSignatureChangeEditor(state, "A variadic signature needs at least one parameter.");
      return;
    }

    const generation = ++inspectorRequestGeneration;
    replaceInspector([
      element("p", "Signature change", "eyebrow"),
      element("h2", callableHeading(state.inspection)),
      element("p", "Analyzing…", "muted"),
    ]);
    try {
	      const result = await postJSON("/api/signature-impact", {
        callable: state.inspection.node.id,
        parameters: parameters.map((type) => ({ type })),
	        results: results.map((type) => ({ type })),
        variadic: state.proposal.variadic,
      });
      if (generation !== inspectorRequestGeneration) return;
	      renderSignatureImpact({ ...state, kind: "signature-result", result });
    } catch (error) {
      if (generation !== inspectorRequestGeneration) return;
	      renderSignatureChangeEditor(state, error.message);
    }
  }

  function renderCallSiteProblem(problem) {
    if (problem.kind === "argument count") return "argument count: " + problem.actualCount + " → " + problem.expectedCount;
    if (problem.kind === "argument type") return "argument " + problem.argument + ": " + problem.actual.display + " → " + problem.expected.display;
    if (problem.kind === "variadic call") return "variadic argument compatibility changed";
    return problem.argument ? "argument " + problem.argument + ": compatibility unknown" : "compatibility unknown";
  }

  function compactLocation(location) {
    if (!location || !location.file) return "unknown location";
    const components = location.file.split(/[\\/]/);
    return components[components.length - 1] + (location.line && location.column ? ":" + location.line + ":" + location.column : "");
  }

  function renderImpactCallSite(site) {
    const item = element("li", undefined, "impact-row");
    item.appendChild(renderSymbolButton(site.caller));
    (site.problems || []).forEach((problem) => item.appendChild(element("span", renderCallSiteProblem(problem), "impact-detail")));
    item.appendChild(element("span", compactLocation(site.location), "evidence"));
    return item;
  }

  function impactList(title, values, render) {
    const parts = [element("h4", title)];
    const list = element("ul");
    values.forEach((value) => list.appendChild(render(value)));
    parts.push(list);
    return parts;
  }

  function renderCallSiteImpact(result) {
    const sites = result.callSites || [];
    const incompatible = sites.filter((site) => site.compatibility === "incompatible");
    const compatible = sites.filter((site) => site.compatibility === "compatible");
    const unknown = sites.filter((site) => site.compatibility === "unknown");
    const parts = [
      element("h3", "Call-site impact"),
      element("p", incompatible.length + " incompatible · " + compatible.length + " compatible · " + unknown.length + " unknown", "impact-summary"),
    ];
    if (incompatible.length > 0) parts.push(...impactList("Incompatible", incompatible, renderImpactCallSite));
    if (unknown.length > 0) parts.push(...impactList("Unknown", unknown, renderImpactCallSite));
    if (compatible.length > 0) {
      const list = element("ul");
      compatible.forEach((site) => list.appendChild(renderImpactCallSite(site)));
      parts.push(disclosure("Show compatible call sites (" + compatible.length + ")", [list], "impact-disclosure"));
    }
    return parts;
  }

  function renderContractImpact(result) {
    const contracts = result.contracts || [];
    const parts = [element("h3", "Contract impact")];
    if (contracts.length === 0) {
      parts.push(element("p", "None", "muted"));
      return parts;
    }
    parts.push(element("h4", "Lost implementation"));
    const list = element("ul");
    contracts.forEach((contract) => {
      const item = element("li", undefined, "impact-row impact-sentence");
      item.append(renderSymbolButton(contract.concrete), document.createTextNode(" no longer implements "), renderSymbolButton(contract.interface));
      if (contract.concreteMethod.ref && contract.interfaceMethod.ref) {
        const methods = element("div", undefined, "impact-methods");
        methods.append(renderSymbolButton(contract.concreteMethod), document.createTextNode(" · "), renderSymbolButton(contract.interfaceMethod));
        item.appendChild(methods);
      }
      list.appendChild(item);
    });
    parts.push(list);
    return parts;
  }

  function renderCompilerImpact(result) {
    const compiler = result.compiler || { consequences: [], affectedPackages: [], baselineStatus: "unknown" };
    const consequences = compiler.consequences || [];
    const parts = [element("h3", "Compiler consequences")];
    if (compiler.baselineStatus === "has diagnostics") {
      parts.push(element("p", "The affected scope already had compiler diagnostics; moved or duplicated diagnostics may be uncertain.", "muted"));
    }
    if (consequences.length === 0) {
      parts.push(element("p", "No new compile/type-check diagnostics were observed in the affected scope.", "muted"));
      return parts;
    }
    const list = element("ul");
    consequences.forEach((consequence) => {
      const item = element("li", undefined, "impact-row");
      if (consequence.symbol && consequence.symbol.ref) {
        item.appendChild(renderSymbolButton(consequence.symbol));
      } else {
        item.appendChild(element("span", consequence.package, "impact-symbol"));
      }
      item.appendChild(element("span", consequence.message, "impact-detail"));
      item.appendChild(element("span", consequence.classification === "uncertain" ? "Uncertain · " + compactLocation(consequence.location) : compactLocation(consequence.location), "evidence"));
      list.appendChild(item);
    });
    parts.push(list);
    return parts;
  }

  function renderStructuralImpact(result) {
    const impacts = result.structural || [];
    const parts = [element("h3", "Structural impact")];
    if (impacts.length === 0) {
      parts.push(element("p", "None", "muted"));
      return parts;
    }
    parts.push(element("h4", "Promoted method changed"));
    const list = element("ul");
    impacts.forEach((structural) => {
      const item = element("li", undefined, "impact-row impact-sentence");
      const exposedType = structural.exposure === "pointer only" ? { ...structural.type, name: "*" + structural.type.name } : structural.type;
      item.append(renderSymbolButton(exposedType), document.createTextNode(" exposes " + structural.originMethod.name + " from "), renderSymbolButton(structural.originMethod));
      list.appendChild(item);
    });
    parts.push(list);
    return parts;
  }

  function renderSignatureImpact(state) {
    inspectorState = state;
    const heading = callableHeading(state.inspection);
    const parts = [
	      element("p", "Proposed signature change", "eyebrow"),
      element("h2", heading),
      element("h3", "Before"),
      element("p", formatCallableSignature(heading, state.result.before), "signature-line"),
      element("h3", "After"),
      element("p", formatCallableSignature(heading, state.result.after), "signature-line"),
    ];
    parts.push(...renderCallSiteImpact(state.result));
	    parts.push(...renderCompilerImpact(state.result));
    parts.push(...renderContractImpact(state.result));
    parts.push(...renderStructuralImpact(state.result));
    const actions = element("div", undefined, "workflow-actions result-actions");
    actions.append(
	      renderWorkflowButton("Edit proposed signature", "workflow-button primary", () => {
        invalidateInspectorRequests();
	        renderSignatureChangeEditor({ ...state, kind: "signature-edit" });
      }),
      renderWorkflowButton("Back to " + callableKind(state.inspection), "workflow-button secondary", () => returnToCallableInspection(state.inspection)),
    );
    parts.push(actions);
    replaceInspector(parts);
  }

  function renderFunctionInspection(result) {
    const detail = result.function;
    const method = result.node.parentKind === "struct" || result.node.parentKind === "interface";
    const heading = callableHeading(result);
    inspectorState = { kind: "symbol", inspection: result, proposal: null, result: null };
    const parts = [
      element("p", method ? "Method" : "Function", "eyebrow"),
      element("h2", heading),
      element("p", result.node.id, "symbol identity-line"),
    ];
    if (detail.signature) {
	      parts.push(renderWorkflowButton("Analyze signature change", "workflow-button impact-action", () => beginSignatureChange(result)));
    }
    if (method) {
      const owner = {
        ref: result.node.parent,
        kind: result.node.parentKind,
        name: result.node.parentName,
        parentRef: "",
        parentName: "",
      };
      parts.push(element("h3", "Owner"));
      const ownerList = element("ul");
      ownerList.appendChild(symbolItem(owner));
      parts.push(ownerList);
    }
    parts.push(
      element("h3", "Documentation"),
      element("p", result.node.documentation || "No function documentation.", result.node.documentation ? "documentation" : "muted"),
    );

    const relationshipParts = [];
    relationshipParts.push(...nonEmptySection("Calls", detail.calls, (value) => symbolRelationshipItem(value, false)));
    relationshipParts.push(...nonEmptySection("Called by", detail.calledBy, (value) => symbolRelationshipItem(value, true)));
    relationshipParts.push(...nonEmptySection("Accepts", detail.accepts, (value) => symbolRelationshipItem(value, false)));
    relationshipParts.push(...nonEmptySection("Returns", detail.returns, (value) => symbolRelationshipItem(value, false)));
    relationshipParts.push(...nonEmptySection("Implements", detail.implements, (value) => symbolRelationshipItem(value, false)));
    relationshipParts.push(...nonEmptySection("Implemented by", detail.implementedBy, (value) => symbolRelationshipItem(value, true)));
    if (relationshipParts.length === 0) {
      parts.push(element("p", "No semantic relationships.", "muted semantic-empty"));
    } else {
      parts.push(...relationshipParts);
    }
    replaceInspector(parts);
  }

  function renderNodeInspection(result) {
    if (result.package) {
      renderPackageInspection(result);
    } else if (result.type) {
      renderTypeInspection(result);
    } else if (result.function) {
      renderFunctionInspection(result);
    }
  }

  async function inspectSymbol(ref) {
    const generation = beginInspectorRequest(typeLabel(ref));
    try {
      const result = await getJSON("/api/node?id=" + encodeURIComponent(ref));
      if (generation !== inspectorRequestGeneration) return;
      renderNodeInspection(result);
    } catch (error) {
      if (generation !== inspectorRequestGeneration) return;
      showError(error);
    }
  }

  function showTypeDependency(dependency) {
    invalidateInspectorRequests();
    const facts = dependency.evidence || [];
    replaceInspector([
      element("p", "Type dependency", "eyebrow"),
      element("h2", dependency.semanticFrom + " depends on " + dependency.semanticTo),
      element("p", facts.length + " exact facts", "documentation"),
      collapsibleRelationships(facts, 2, "Show evidence"),
    ]);
  }

  function showPackageGraph() {
    invalidateInspectorRequests();
    window.clearTimeout(typeFreezeFallback);
    if (typeNetwork) {
      typeNetwork.destroy();
      typeNetwork = null;
    }
    typeNetworkElement.hidden = true;
    typeContext.hidden = true;
    networkElement.hidden = false;
    packageSearchControl.setEnabled(true);
    packageNetwork.redraw();
    if (currentPackageDependency) {
      renderPackageDependencyInspection(currentPackageDependency.result);
    }
  }

  backToPackages.addEventListener("click", showPackageGraph);

  function showError(error) {
    replaceInspector([
      element("p", "Inspector", "eyebrow"),
      element("h2", "Unable to inspect selection"),
      element("p", error.message, "error"),
    ]);
  }

  function renderAnalysisStatus(result) {
    if (result.status !== "partial") return;
    const affectedPackages = (result.reasons || []).map((reason) => reason.package);
    analysisStatusIndicator.textContent = "Partial analysis";
    analysisStatusIndicator.title = "Some semantic relationships may be missing because type information was incomplete" +
      (affectedPackages.length > 0 ? " in: " + affectedPackages.join(", ") : ".");
    analysisStatusIndicator.hidden = false;
  }

  getJSON("/api/status").then(renderAnalysisStatus).catch(() => {});

  getJSON("/api/packages").then((data) => {
    const packageNodes = data.nodes.slice();
    packageLabels = new Map(packageNodes.map((node) => [node.id, node.label]));
    const nodes = new vis.DataSet(packageNodes.map((node) => ({
      id: node.id,
      label: node.label,
      title: node.id,
      shape: "box",
      margin: 12,
    })));
    packageEdges = new vis.DataSet(data.edges.map((edge) => ({
      id: edge.id,
      from: edge.to,
      to: edge.from,
      semanticFrom: edge.from,
      semanticTo: edge.to,
      arrows: { to: { enabled: true, scaleFactor: 0.7 } },
    })));
    packageNetwork = new vis.Network(networkElement, { nodes, edges: packageEdges }, {
      layout: { randomSeed: 240519, improvedLayout: true },
      physics: {
        enabled: true,
        stabilization: { enabled: true, iterations: 600, fit: true },
        barnesHut: { gravitationalConstant: -4200, springLength: 170, springConstant: 0.035 },
      },
      interaction: { dragNodes: true, dragView: true, hover: true, hoverConnectedEdges: true, multiselect: false, selectConnectedEdges: true, zoomView: true },
      nodes: {
        borderWidth: 1,
        color: { background: "#ffffff", border: "#64748b", highlight: { background: "#cffafe", border: "#0891b2" } },
        font: { color: "#172033", face: "system-ui", size: 14 },
      },
      edges: {
        color: { color: "#38bdf8", highlight: "#0369a1", hover: "#0284c7" },
        width: 1.5,
        selectionWidth: 2,
        smooth: { enabled: true, type: "dynamic" },
      },
    });

    let physicsFrozen = false;
    let freezeFallback;
    const freezePhysics = () => {
      if (physicsFrozen) return;
      physicsFrozen = true;
      window.clearTimeout(freezeFallback);
      packageNetwork.stopSimulation();
      packageNetwork.setOptions({ physics: false });
    };
    packageNetwork.once("stabilizationIterationsDone", freezePhysics);
    packageNetwork.once("stabilized", freezePhysics);
    freezeFallback = window.setTimeout(freezePhysics, 8000);
    packageSearchControl = setupPackageSearch(packageNodes);

    packageNetwork.on("click", (params) => {
      if (params.nodes.length > 0) {
        selectPackage(params.nodes[0], false);
      } else if (params.edges.length > 0) {
        const selected = packageEdges.get(params.edges[0]);
        openPackageDependency(selected.semanticFrom, selected.semanticTo, selected, false, null);
      }
    });
  }).catch(showError);
}());
