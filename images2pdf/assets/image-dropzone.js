import styles from '/assets/image-dropzone.css' with { type: 'css' };

const response = await fetch('/assets/image-dropzone.html');
const template = document.createElement("template");
template.innerHTML = await response.text();

class ImageDropzone extends HTMLElement {
  constructor() {
    super();
    // Attach Shadow DOM to isolate CSS
    this.attachShadow({ mode: 'open' });
    this.shadowRoot.appendChild(template.content.cloneNode(true));
    this.shadowRoot.adoptedStyleSheets = [styles];

    // Cache DOM elements for easy access
    this.dropArea = this.shadowRoot.getElementById('drop-area');
    this.overlayEl = this.shadowRoot.getElementById('overlay');
    this.fileInput = this.shadowRoot.getElementById('file-input');
    this.selectBtn = this.shadowRoot.getElementById('select-btn');
    this.previewSection = this.shadowRoot.getElementById('preview-section');
    this.previewContainer = this.shadowRoot.getElementById('upload-preview');
    this.mergeBtn = this.shadowRoot.getElementById('merge-btn');

    // Create a reactive state using a Proxy
    // When you change state.isDragging or state.files, the UI updates automatically
    this.state = new Proxy({
      isDragging: false,
      files: []
    }, {
      set: (target, property, value) => {
        target[property] = value;
        
        // React to state changes
        if (property === 'isDragging') {
          this.overlayEl.style.display = value ? 'flex' : 'none';
        }
        
        if (property === 'files') {
          const dt = new DataTransfer();
          value.forEach(file => dt.items.add(file));
          this.fileInput.files = dt.files;
          this.previewSection.style.display = value.length > 0 ? 'flex' : 'none';
          this.renderPreviews(value);
        }

        return true;
      }
    });
  }

  connectedCallback() {
    // This fires when the component is added to the DOM. Setup event listeners here.
    
    this.selectBtn.addEventListener('click', () => {
      this.fileInput.click();
    });

    this.fileInput.addEventListener('change', (e) => {
      this.handleNewFiles(e.target.files);
    });

    // Drag and Drop Events
    window.addEventListener('dragover', (e) => {
      e.preventDefault();
      this.state.isDragging = true;
    });

    window.addEventListener('dragleave', (e) => {
      if (e.clientX === 0 || e.clientY === 0) {
        this.state.isDragging = false;
      }
    });

    window.addEventListener('drop', (e) => {
      e.preventDefault();
      this.state.isDragging = false;
      
      if (e.dataTransfer.files) {
        this.handleNewFiles(e.dataTransfer.files);
      }
    });

    this.mergeBtn.addEventListener('click', async () => {
      const formData = new FormData();
      this.state.files.forEach(f => formData.append('images', f));

      // Change button text to show loading state
      const originalText = this.mergeBtn.innerText;
      this.mergeBtn.innerText = 'Merging...';
      this.mergeBtn.disabled = true;

      try {
        const res = await fetch('/api/merge', { method: 'POST', body: formData });
        if (!res.ok) {
          alert('Failed to merge images');
          return;
        }

        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'merged.pdf';
        a.click();

        this.state.files = [];
      } 

      catch (err) {
        console.error("Upload error:", err)
      } 

      finally {
        this.mergeBtn.innerText = originalText;
        this.mergeBtn.disabled = false;
      }
    })
  }

  handleNewFiles(newFiles) {
    const fileKey = (f) => `${f.name}-${f.size}-${f.lastModified}`;
    const existingKeys = new Set(this.state.files.map(fileKey));
    const updatedFiles = [...this.state.files];

    for (let file of newFiles) {
      const key = fileKey(file);
      if (!['image/jpeg', 'image/png'].includes(file.type) || existingKeys.has(key)) {
        continue;
      }
      existingKeys.add(key);
      updatedFiles.push(file);
    }

    // Trigger the Proxy setter
    this.state.files = updatedFiles;
  }

  renderPreviews(files) {
    this.previewContainer.innerHTML = '';
    files.forEach((file, index) => {
      const item = document.createElement("div");
      item.className = 'preview-item';

      const img = document.createElement("img");
      img.src = URL.createObjectURL(file);

      const btn = document.createElement("button");
      btn.className = 'remove-btn';
      btn.innerText = 'x';

      btn.onclick = () => {
        const newFiles = [...this.state.files];
        newFiles.splice(index, 1);
        this.state.files = newFiles;
      };

      item.appendChild(img);
      item.appendChild(btn);
      this.previewContainer.appendChild(item);
    });
  }
}

// Register the custom element with the browser
customElements.define('x-image-dropzone', ImageDropzone);
