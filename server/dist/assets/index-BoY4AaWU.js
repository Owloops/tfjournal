(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const i of document.querySelectorAll('link[rel="modulepreload"]'))l(i);new MutationObserver(i=>{for(const a of i)if(a.type==="childList")for(const c of a.addedNodes)c.tagName==="LINK"&&c.rel==="modulepreload"&&l(c)}).observe(document,{childList:!0,subtree:!0});function n(i){const a={};return i.integrity&&(a.integrity=i.integrity),i.referrerPolicy&&(a.referrerPolicy=i.referrerPolicy),i.crossOrigin==="use-credentials"?a.credentials="include":i.crossOrigin==="anonymous"?a.credentials="omit":a.credentials="same-origin",a}function l(i){if(i.ep)return;i.ep=!0;const a=n(i);fetch(i.href,a)}})();function N(e,t){let n;return(...l)=>{clearTimeout(n),n=setTimeout(()=>e(...l),t)}}const s={runs:[],filteredRuns:[],selectedRunId:null,selectedRun:null,selectedIndex:-1,currentView:"details",searchQuery:"",statusFilter:"",sinceFilter:"",programFilter:"",branchFilter:"",hasChangesFilter:!1,limit:20,focusPanel:"runs",showHelp:!1},v=document.getElementById("runsList"),E=document.getElementById("contentBody"),o=document.getElementById("searchInput"),h=document.getElementById("filterBtn"),L=document.getElementById("filterCount"),f=document.getElementById("filterPopover"),R=document.getElementById("filterChips"),C=document.getElementById("statusFilter"),D=document.getElementById("sinceFilter"),S=document.getElementById("programFilter"),M=document.getElementById("branchFilter"),B=document.getElementById("hasChangesFilter"),_=document.getElementById("limitFilter"),x=document.querySelectorAll(".tab"),y=document.getElementById("helpModal");async function O(){const e=new URLSearchParams;s.statusFilter&&e.set("status",s.statusFilter),s.sinceFilter&&e.set("since",s.sinceFilter),s.programFilter&&e.set("program",s.programFilter),s.branchFilter&&e.set("branch",s.branchFilter),s.hasChangesFilter&&e.set("has-changes","true"),s.limit&&e.set("limit",s.limit.toString());const t=await fetch(`/api/runs?${e}`);if(!t.ok)throw new Error("Failed to fetch runs");return t.json()}async function A(e){const t=await fetch(`/api/runs/${e}`);if(!t.ok)throw new Error("Failed to fetch run");return t.json()}async function U(e){const t=await fetch(`/api/runs/${e}/output`);return t.ok?t.text():null}async function Q(){try{const e=await fetch("/api/version");return e.ok?(await e.json()).version:null}catch{return null}}function T(e){return new Date(e).toLocaleDateString("en-US",{month:"short",day:"numeric",hour:"2-digit",minute:"2-digit"})}function w(e){if(!e)return"-";const t=Math.floor(e/1e3),n=Math.floor(t/60),l=t%60;return n>0?`${n}m ${l}s`:`${t}s`}function P(e){if(!e)return"";const t=[];return e.add>0&&t.push(`+${e.add}`),e.change>0&&t.push(`~${e.change}`),e.destroy>0&&t.push(`-${e.destroy}`),t.join(" ")||"+0"}function q(e){switch(e){case"local":return'<span class="sync-icon sync-local" title="Local only (not synced to S3)">↑</span>';case"remote":return'<span class="sync-icon sync-remote" title="Remote only (not downloaded)">↓</span>';case"synced":return'<span class="sync-icon sync-synced" title="Synced">✓</span>';default:return""}}function H(){const e=s.searchQuery.toLowerCase();s.filteredRuns=s.runs.filter(t=>!(e&&!t.workspace.toLowerCase().includes(e)&&!t.user?.toLowerCase().includes(e)&&!t.id.toLowerCase().includes(e)))}function $(){if(s.filteredRuns.length===0){v.innerHTML='<div class="empty-state"><p>No runs found</p></div>';return}v.innerHTML=s.filteredRuns.map((e,t)=>`
    <div class="run-item ${e.id===s.selectedRunId?"selected":""}" data-id="${e.id}" data-index="${t}">
      <div class="run-item-header">
        <div class="run-status ${e.status}"></div>
        <div class="run-workspace">${r(e.workspace)}</div>
        <div class="run-changes">${P(e.changes)}${q(e.sync_status)}</div>
      </div>
      <div class="run-item-meta">
        <span class="run-time">${T(e.timestamp)}</span>
        <span class="run-user">${r(e.user||"unknown")}</span>
      </div>
    </div>
  `).join("")}function K(e){const t=e.git||{},n=e.ci||{};return`
    <div class="details-view">
      <div class="detail-section">
        <div class="detail-section-title">Run Info</div>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">ID</span>
            <span class="detail-value">${r(e.id)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Status</span>
            <span class="badge badge-${e.status==="success"?"success":"error"}">${e.status}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Workspace</span>
            <span class="detail-value">${r(e.workspace)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Duration</span>
            <span class="detail-value">${w(e.duration_ms)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Program</span>
            <span class="detail-value">${r(e.program||"terraform")}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">User</span>
            <span class="detail-value">${r(e.user||"unknown")}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Timestamp</span>
            <span class="detail-value">${T(e.timestamp)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Changes</span>
            <span class="detail-value">${P(e.changes)}</span>
          </div>
        </div>
      </div>

      ${t.commit?`
      <div class="detail-section">
        <div class="detail-section-title">Git</div>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">Commit</span>
            <span class="detail-value">${r(t.commit)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Branch</span>
            <span class="detail-value">${r(t.branch||"-")}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Dirty</span>
            <span class="detail-value">${t.dirty?"Yes":"No"}</span>
          </div>
          ${t.message?`
          <div class="detail-item">
            <span class="detail-label">Message</span>
            <span class="detail-value">${r(t.message)}</span>
          </div>
          `:""}
        </div>
      </div>
      `:""}

      ${n.provider?`
      <div class="detail-section">
        <div class="detail-section-title">CI</div>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">Provider</span>
            <span class="detail-value">${r(n.provider)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Actor</span>
            <span class="detail-value">${r(n.actor||"-")}</span>
          </div>
          ${n.workflow?`
          <div class="detail-item">
            <span class="detail-label">Workflow</span>
            <span class="detail-value">${r(n.workflow)}</span>
          </div>
          `:""}
        </div>
      </div>
      `:""}

      ${e.resources&&e.resources.length>0?`
      <div class="detail-section">
        <div class="detail-section-title">Resources (${e.resources.length})</div>
        <div style="max-height: 200px; overflow-y: auto;">
          ${e.resources.map(l=>`
            <div style="font-family: var(--font-mono); font-size: 0.8125rem; padding: 0.25rem 0; color: var(--color-text-secondary);">
              <span class="resource-action ${l.action}" style="margin-right: 0.5rem;">${l.action==="create"?"+":l.action==="destroy"?"-":"~"}</span>
              ${r(l.address)}
            </div>
          `).join("")}
        </div>
      </div>
      `:""}
    </div>
  `}function G(e){switch(e){case"success":return'<span class="status-icon success">✓</span>';case"failed":return'<span class="status-icon error">✗</span>';default:return'<span class="status-icon pending">●</span>'}}function W(e){return!e.resources||e.resources.length===0?'<div class="empty-state"><p>No resource events</p></div>':`
    <table class="events-table">
      <thead>
        <tr>
          <th>Action</th>
          <th>Resource</th>
          <th>Duration</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        ${e.resources.map(t=>`
          <tr>
            <td><span class="resource-action ${t.action}">${t.action}</span></td>
            <td class="resource-address">${r(t.address)}</td>
            <td class="resource-duration">${w(t.duration_ms)}</td>
            <td class="resource-status">${G(t.status)}</td>
          </tr>
        `).join("")}
      </tbody>
    </table>
  `}function z(e){if(!e.resources||e.resources.length===0)return'<div class="empty-state"><p>No resource timeline</p></div>';const t=e.resources.filter(a=>a.duration_ms>0&&a.start_time);if(t.length===0)return'<div class="empty-state"><p>No timing data available</p></div>';let n=1/0,l=0;for(const a of t){const c=new Date(a.start_time).getTime(),g=c+a.duration_ms;c<n&&(n=c),g>l&&(l=g)}const i=l-n||1;return`
    <div class="timeline-view">
      <div class="gantt-header">
        <div class="gantt-header-label">Resource</div>
        <div class="gantt-header-bar">
          <span>0s</span>
          <span>${w(i)}</span>
        </div>
      </div>
      <div class="gantt-chart">
        ${t.map(a=>{const k=(new Date(a.start_time).getTime()-n)/i*100,j=Math.min(Math.max(a.duration_ms/i*100,.5),100-k);return`
            <div class="gantt-row">
              <div class="gantt-label" title="${r(a.address)}">${r(a.address)}</div>
              <div class="gantt-bar-container">
                <div class="gantt-bar ${a.action}" style="left: ${k}%; width: ${j}%;"></div>
              </div>
            </div>
          `}).join("")}
      </div>
    </div>
  `}async function Y(e){const t=await U(e.id);return t?`
    <div class="output-view">
      <pre class="output-content">${r(t)}</pre>
    </div>
  `:'<div class="empty-state"><p>No output available</p></div>'}async function F(){if(!s.selectedRun){E.innerHTML='<div class="empty-state"><p>Select a run to view details</p></div>';return}let e="";switch(s.currentView){case"details":e=K(s.selectedRun);break;case"events":e=W(s.selectedRun);break;case"timeline":e=z(s.selectedRun);break;case"output":e=await Y(s.selectedRun);break}E.innerHTML=e}function r(e){if(!e)return"";const t=document.createElement("div");return t.textContent=e,t.innerHTML}async function V(e,t){s.selectedRunId=e,s.selectedIndex=t,s.selectedRun=await A(e),$(),await F()}function m(e){if(e<0||e>=s.filteredRuns.length)return;const t=s.filteredRuns[e];V(t.id,e)}function p(e){s.currentView=e,x.forEach(t=>{t.classList.toggle("active",t.dataset.view===e)}),F()}async function d(){v.innerHTML='<div class="loading">Loading runs...</div>';try{s.runs=await O()||[]}catch{s.runs=[],v.innerHTML='<div class="empty-state"><p>Failed to load runs</p></div>';return}H(),$(),s.filteredRuns.length>0?s.selectedRunId&&s.filteredRuns.some(t=>t.id===s.selectedRunId)||m(0):(s.selectedRunId=null,s.selectedRun=null,s.selectedIndex=-1,F())}function I(e){if(s.filteredRuns.length===0)return;let t=s.selectedIndex+e;t<0&&(t=0),t>=s.filteredRuns.length&&(t=s.filteredRuns.length-1),t!==s.selectedIndex&&m(t)}function b(){s.showHelp=!s.showHelp,y.classList.toggle("visible",s.showHelp)}function J(){f.classList.toggle("visible"),h.classList.toggle("active",f.classList.contains("visible"))}function X(){f.classList.remove("visible"),h.classList.remove("active")}function Z(){const e=[];return s.statusFilter&&e.push({key:"status",value:s.statusFilter,label:`status:${s.statusFilter}`}),s.sinceFilter&&e.push({key:"since",value:s.sinceFilter,label:`since:${s.sinceFilter}`}),s.programFilter&&e.push({key:"program",value:s.programFilter,label:`program:${s.programFilter}`}),s.branchFilter&&e.push({key:"branch",value:s.branchFilter,label:`branch:${s.branchFilter}`}),s.hasChangesFilter&&e.push({key:"hasChanges",value:!0,label:"has-changes"}),e}function u(){const e=Z();L.textContent=e.length,L.classList.toggle("visible",e.length>0),R.innerHTML=e.map(t=>`
    <span class="filter-chip" data-key="${t.key}">
      ${t.label}
      <span class="filter-chip-remove">×</span>
    </span>
  `).join("")}function ee(e){switch(e){case"status":s.statusFilter="",C.value="";break;case"since":s.sinceFilter="",D.value="";break;case"program":s.programFilter="",S.value="";break;case"branch":s.branchFilter="",M.value="";break;case"hasChanges":s.hasChangesFilter=!1,B.checked=!1;break}u(),d()}function te(e){if(s.showHelp){(e.key==="?"||e.key==="Escape")&&(b(),e.preventDefault());return}const t=document.activeElement===o,n=document.activeElement.tagName==="SELECT";if(t){e.key==="Escape"&&(o.blur(),s.searchQuery===""&&(s.focusPanel="runs"),e.preventDefault());return}if(n){e.key==="Escape"&&(document.activeElement.blur(),e.preventDefault());return}switch(e.key){case"/":o.focus(),e.preventDefault();break;case"j":case"ArrowDown":I(1),e.preventDefault();break;case"k":case"ArrowUp":I(-1),e.preventDefault();break;case"g":e.shiftKey||(m(0),e.preventDefault());break;case"G":m(s.filteredRuns.length-1),e.preventDefault();break;case"d":p("details"),e.preventDefault();break;case"e":p("events"),e.preventDefault();break;case"t":p("timeline"),e.preventDefault();break;case"o":p("output"),e.preventDefault();break;case"?":b(),e.preventDefault();break;case"Escape":s.focusPanel="runs";break}}o.addEventListener("input",e=>{s.searchQuery=e.target.value,H(),$()});o.addEventListener("focus",()=>{o.placeholder="Search runs..."});o.addEventListener("blur",()=>{o.placeholder="/ to search..."});h.addEventListener("click",e=>{e.stopPropagation(),J()});f.addEventListener("click",e=>{e.stopPropagation()});R.addEventListener("click",e=>{const t=e.target.closest(".filter-chip");t&&ee(t.dataset.key)});document.addEventListener("click",e=>{!h.contains(e.target)&&!f.contains(e.target)&&X()});C.addEventListener("change",e=>{s.statusFilter=e.target.value,u(),d()});D.addEventListener("change",e=>{s.sinceFilter=e.target.value,u(),d()});S.addEventListener("change",e=>{s.programFilter=e.target.value,u(),d()});M.addEventListener("input",N(e=>{s.branchFilter=e.target.value,u(),d()},300));B.addEventListener("change",e=>{s.hasChangesFilter=e.target.checked,u(),d()});_.addEventListener("change",e=>{s.limit=parseInt(e.target.value,10),d()});v.addEventListener("click",e=>{const t=e.target.closest(".run-item");if(t){const n=parseInt(t.dataset.index,10);V(t.dataset.id,n)}});x.forEach(e=>{e.addEventListener("click",()=>{p(e.dataset.view)})});y.addEventListener("click",e=>{e.target===y&&b()});document.addEventListener("keydown",te);async function se(){const e=await Q(),t=document.getElementById("footerVersion");e&&t&&(t.textContent=e)}d();se();
