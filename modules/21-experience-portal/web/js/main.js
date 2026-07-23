// OPERAN — Shared JavaScript
// ═══════════════════════════════════════════════════════════
// DATA VISUALIZATION NETWORK (Real-time event stream — Canvas 2D)
// ═══════════════════════════════════════════════════════════
const OperanDataVisual = {
  canvas: null, ctx: null, running: false,
  rows: [],

  // ── Simulated Operan platform data ──────────────────────────
  _modules: [
    { id: 'M01', name: 'Tenant CP', color: '#00B4D8' },
    { id: 'M02', name: 'IAM', color: '#0077B6' },
    { id: 'M03', name: 'Orchestrator', color: '#00D4FF' },
    { id: 'M04', name: 'Agent Reg', color: '#11999E' },
    { id: 'M05', name: 'Dept Templ', color: '#D4A853' },
    { id: 'M06', name: 'Knowledge', color: '#38B000' },
    { id: 'M07', name: 'Memory', color: '#8338EC' },
    { id: 'M08', name: 'Tool Exec', color: '#FF006E' },
    { id: 'M09', name: 'Supervise', color: '#3A86FF' },
    { id: 'M10', name: 'Policy', color: '#FB5607' },
    { id: 'M11', name: 'Observability', color: '#06D6A0' },
    { id: 'M12', name: 'Model Abs', color: '#FFD166' },
    { id: 'M13', name: 'Model Route', color: '#EF476F' },
    { id: 'M14', name: 'Collaboration', color: '#118AB2' },
    { id: 'M15', name: 'Marketplace', color: '#073B4C' },
    { id: 'M16', name: 'Sandbox', color: '#94D2BD' },
    { id: 'M17', name: 'Cost Gov', color: '#A8DADC' },
    { id: 'M18', name: 'Connectors', color: '#E63946' },
    { id: 'M19', name: 'Arabic NLP', color: '#F4A261' },
    { id: 'M20', name: 'Sovereign', color: '#2B2D42' }
  ],
  _eventTypes: [
    'CREATE', 'UPDATE', 'DEPLOY', 'SCALED', 'APPROVED',
    'AUDIT', 'SCAN', 'SYNC', 'ROUTED', 'EXECUTED'
  ],
  _statuses: ['success', 'success', 'success', 'success', 'success', 'warning', 'error'],

  init(canvasId, options) {
    options = options || {};
    var rowCount = options.rowCount || 5;

    this.canvas = document.getElementById(canvasId);
    if (!this.canvas) return;
    this.ctx = this.canvas.getContext('2d');
    if (!this.ctx) return;
    this.running = true;

    var self = this;
    this.canvas.addEventListener('mousemove', function(e) {
      var rect = self.canvas.getBoundingClientRect();
      self.mouse = { x: e.clientX - rect.left, y: e.clientY - rect.top };
    });
    this.canvas.addEventListener('mouseleave', function() {
      self.mouse = { x: -9999, y: -9999 };
    });

    // Card dimensions
    var cardW = 180;
    var gap = 8;
    var cardSpace = cardW + gap;

    // Only place 4-5 cards per row initially
    var initialCards = 4;

    // Create rows with independent speeds
    this.rows = [];
    for (var r = 0; r < rowCount; r++) {
      var row = {
        y: 0,
        speed: 0.35 + r * 0.04,
        events: [],
        cardSpace: cardSpace
      };

      // Place cards sequentially with fixed gaps
      for (var i = 0; i < initialCards; i++) {
        var evt = self._spawnEvent();
        evt.x = i * 238; // matches draw card width (230) + gap (8)
        row.events.push(evt);
      }

      this.rows.push(row);
    }

    self._resize();
    window.addEventListener('resize', function() { self._resize(); });

    self._animate();
  },

  _spawnEvent: function() {
    var mod = this._modules[Math.floor(Math.random() * this._modules.length)];
    var type = this._eventTypes[Math.floor(Math.random() * this._eventTypes.length)];
    var status = this._statuses[Math.floor(Math.random() * this._statuses.length)];
    var now = new Date();
    return {
      module: mod,
      type: type,
      status: status,
      timestamp: now,
      x: window.innerWidth + 50
    };
  },

  _formatTime: function(d) {
    return d.getHours().toString().padStart(2, '0') + ':' +
           d.getMinutes().toString().padStart(2, '0') + ':' +
           d.getSeconds().toString().padStart(2, '0') + '.' +
           d.getMilliseconds().toString().padStart(3, '0');
  },

  _resize: function() {
    var canvas = this.canvas;
    var dpr = window.devicePixelRatio || 1;
    canvas.width = window.innerWidth * dpr;
    canvas.height = window.innerHeight * dpr;
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    var rowH = window.innerHeight / this.rows.length;
    for (var i = 0; i < this.rows.length; i++) {
      this.rows[i].y = rowH * i + rowH / 2;
      this.rows[i].height = rowH;
    }
  },

  _animate: function() {
    if (!this.running) return;
    var ctx = this.ctx;
    var w = window.innerWidth;
    var h = window.innerHeight;
    var now = Date.now();

    ctx.clearRect(0, 0, w, h);

    // Draw subtle row guides
    for (var r = 0; r < this.rows.length; r++) {
      var row = this.rows[r];
      ctx.strokeStyle = 'rgba(0, 180, 216, 0.04)';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(0, row.y);
      ctx.lineTo(w, row.y);
      ctx.stroke();
    }

    // Update and draw events
    for (var r = 0; r < this.rows.length; r++) {
      this._updateRow(ctx, this.rows[r], w, now);
    }

    requestAnimationFrame(function() { this._animate(); }.bind(this));
  },

  _updateRow: function(ctx, row, w, now) {
    // Remove events that scrolled off the left
    row.events = row.events.filter(function(e) {
      return e.x > -500;
    });

    // Move events left
    for (var i = 0; i < row.events.length; i++) {
      row.events[i].x -= row.speed;
      row.events[i].timestamp = new Date(now);
    }

    // Spawn new events at the right edge of the row
    var lastEvent = row.events[row.events.length - 1];
    if (!lastEvent || lastEvent.x < w - 200) {
      var evt = this._spawnEvent();
      // Position after last event (card width 230 + gap 8)
      evt.x = lastEvent ? lastEvent.x + 238 : w;
      row.events.push(evt);
    }

    this._drawRowEvents(ctx, row);
  },

  _drawRowEvents: function(ctx, row) {
    for (var i = 0; i < row.events.length; i++) {
      var e = row.events[i];
      if (e.x < -200 || e.x > window.innerWidth + 100) continue;

      var y = row.y - 16;
      var h = 32;
      var eventW = 230;
      var rx = e.x;
      var ry = y;

      // ── Event card background ──
      var statusColor = e.status === 'success' ? '0, 180, 216' :
                        e.status === 'warning' ? '212, 168, 83' : '255, 0, 50';
      ctx.fillStyle = 'rgba(' + statusColor + ', 0.08)';
      ctx.strokeStyle = 'rgba(' + statusColor + ', 0.2)';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(rx + 6, ry);
      ctx.lineTo(rx + eventW - 6, ry);
      ctx.quadraticCurveTo(rx + eventW, ry, rx + eventW, ry + 6);
      ctx.lineTo(rx + eventW, ry + h - 6);
      ctx.quadraticCurveTo(rx + eventW, ry + h, rx + eventW - 6, ry + h);
      ctx.lineTo(rx + 6, ry + h);
      ctx.quadraticCurveTo(rx, ry + h, rx, ry + h - 6);
      ctx.lineTo(rx, ry + 6);
      ctx.quadraticCurveTo(rx, ry, rx + 6, ry);
      ctx.closePath();
      ctx.fill();
      ctx.stroke();

      // ── Module badge ──
      var badgeX = rx + 4;
      var badgeY = ry + 4;
      var badgeW = 32;
      var badgeH = h - 8;
      ctx.fillStyle = e.module.color + '30';
      ctx.strokeStyle = e.module.color + '60';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(badgeX + 3, badgeY);
      ctx.lineTo(badgeX + badgeW - 3, badgeY);
      ctx.quadraticCurveTo(badgeX + badgeW, badgeY, badgeX + badgeW, badgeY + 3);
      ctx.lineTo(badgeX + badgeW, badgeY + badgeH - 3);
      ctx.quadraticCurveTo(badgeX + badgeW, badgeY + badgeH, badgeX + badgeW - 3, badgeY + badgeH);
      ctx.lineTo(badgeX + 3, badgeY + badgeH);
      ctx.quadraticCurveTo(badgeX, badgeY + badgeH, badgeX, badgeY + badgeH - 3);
      ctx.lineTo(badgeX, badgeY + 3);
      ctx.quadraticCurveTo(badgeX, badgeY, badgeX + 3, badgeY);
      ctx.closePath();
      ctx.fill();
      ctx.stroke();

      // Module text in badge
      ctx.fillStyle = e.module.color;
      ctx.font = 'bold 9px "JetBrains Mono", monospace';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(e.module.id, badgeX + badgeW / 2, badgeY + badgeH / 2);

      // ── Event type ──
      ctx.fillStyle = 'rgba(240, 244, 248, 0.85)';
      ctx.font = '10px "Inter", sans-serif';
      ctx.textAlign = 'left';
      ctx.fillText(e.type, badgeX + badgeW + 10, badgeY + badgeH / 2);

      // ── Status dot ──
      var dotX = badgeX + badgeW + 80;
      var dotY = badgeY + badgeH / 2;
      ctx.beginPath();
      ctx.arc(dotX + 4, dotY, 3, 0, Math.PI * 2);
      ctx.fillStyle = e.status === 'success' ? '#06D6A0' :
                      e.status === 'warning' ? '#FFD166' : '#FF006E';
      ctx.fill();

      // Status label
      ctx.fillStyle = 'rgba(240, 244, 248, 0.5)';
      ctx.font = '8px "JetBrains Mono", monospace';
      ctx.fillText(e.status.toUpperCase(), dotX + 10, dotY);

      // ── Timestamp ──
      ctx.fillStyle = 'rgba(148, 163, 184, 0.7)';
      ctx.font = '9px "JetBrains Mono", monospace';
      ctx.textAlign = 'right';
      ctx.fillText(this._formatTime(e.timestamp), rx + eventW - 4, badgeY + badgeH / 2);
    }
  },

  stop: function() { this.running = false; }
};

// ═══════════════════════════════════════════════════════════
// NAVIGATION
// ═══════════════════════════════════════════════════════════
const OperanNav = {
  init: function() {
    var nav = document.getElementById('nav');
    if (!nav) return;
    window.addEventListener('scroll', function() {
      nav.classList.toggle('scrolled', window.scrollY > 40);
    });
    var toggle = document.getElementById('navToggle');
    var links = document.getElementById('navLinks');
    if (toggle && links) {
      toggle.addEventListener('click', function() {
        links.classList.toggle('open');
        toggle.classList.toggle('active');
      });
      var linkEls = links.querySelectorAll('.nav-link');
      for (var i = 0; i < linkEls.length; i++) {
        linkEls[i].addEventListener('click', function() {
          links.classList.remove('open');
          toggle.classList.remove('active');
        });
      }
    }
  }
};

// ═══════════════════════════════════════════════════════════
// SCROLL REVEAL
// ═══════════════════════════════════════════════════════════
const OperanReveal = {
  init: function() {
    var obs = new IntersectionObserver(function(entries) {
      for (var i = 0; i < entries.length; i++) {
        if (entries[i].isIntersecting) {
          entries[i].target.classList.add('visible');
          obs.unobserve(entries[i].target);
        }
      }
    }, { threshold: 0.1, rootMargin: '0px 0px -40px 0px' });
    document.querySelectorAll('.reveal, .reveal-left, .reveal-right').forEach(function(el) { obs.observe(el); });
  }
};

// ═══════════════════════════════════════════════════════════
// ANIMATED COUNTERS
// ═══════════════════════════════════════════════════════════
const OperanCounters = {
  init: function() {
    var obs = new IntersectionObserver(function(entries) {
      entries.forEach(function(entry) {
        if (entry.isIntersecting && !entry.target.dataset.counted) {
          entry.target.dataset.counted = '1';
          var target = parseInt(entry.target.dataset.target);
          if (isNaN(target)) return;
          var duration = 2000;
          var start = performance.now();
          function animate(now) {
            var progress = Math.min((now - start) / duration, 1);
            var eased = 1 - Math.pow(1 - progress, 3);
            entry.target.textContent = Math.floor(eased * target).toLocaleString();
            if (progress < 1) requestAnimationFrame(animate);
            else entry.target.textContent = target.toLocaleString();
          }
          requestAnimationFrame(animate);
        }
      });
    }, { threshold: 0.5 });
    document.querySelectorAll('[data-target]').forEach(function(el) { obs.observe(el); });
  }
};

// ═══════════════════════════════════════════════════════════
// 3D CARD TILT
// ═══════════════════════════════════════════════════════════
const Operan3DCards = {
  init: function(sel) {
    sel = sel || '.tilt-card';
    document.querySelectorAll(sel).forEach(function(card) {
      card.style.perspective = '1000px';
      card.style.transformStyle = 'preserve-3d';
      card.addEventListener('mousemove', function(e) {
        var rect = card.getBoundingClientRect();
        var x = (e.clientX - rect.left) / rect.width;
        var y = (e.clientY - rect.top) / rect.height;
        var tiltX = (y - 0.5) * -12;
        var tiltY = (x - 0.5) * 12;
        card.style.transform = 'rotateX(' + tiltX + 'deg) rotateY(' + tiltY + 'deg) scale(1.02)';
        card.style.setProperty('--mx', (x*100)+'%');
        card.style.setProperty('--my', (y*100)+'%');
      });
      card.addEventListener('mouseleave', function() {
        card.style.transform = 'rotateX(0) rotateY(0) scale(1)';
      });
    });
  }
};

// ═══════════════════════════════════════════════════════════
// TABS
// ═══════════════════════════════════════════════════════════
const OperanTabs = {
  init: function(groupId) {
    var container = document.getElementById(groupId);
    if (!container) return;
    var btns = container.querySelectorAll('.tab-btn');
    var panels = container.querySelectorAll('.tab-panel');
    for (var i = 0; i < btns.length; i++) {
      btns[i].addEventListener('click', function() {
        var tabId = this.dataset.tab;
        for (var j = 0; j < btns.length; j++) btns[j].classList.remove('active');
        this.classList.add('active');
        for (var k = 0; k < panels.length; k++) panels[k].classList.remove('active');
        var target = container.querySelector('[data-panel="' + tabId + '"]');
        if (target) target.classList.add('active');
      });
    }
  }
};

// ═══════════════════════════════════════════════════════════
// MAGNETIC CURSOR GLOW
// ═══════════════════════════════════════════════════════════
const OperanCursorGlow = {
  init: function() {
    var glow = document.getElementById('cursor-glow');
    if (!glow) return;
    var x = 0, y = 0, cx = 0, cy = 0;
    document.addEventListener('mousemove', function(e) { x = e.clientX; y = e.clientY; });
    (function animate() {
      cx += (x - cx) * 0.12;
      cy += (y - cy) * 0.12;
      glow.style.left = cx + 'px';
      glow.style.top = cy + 'px';
      requestAnimationFrame(animate);
    })();
  }
};

// ═══════════════════════════════════════════════════════════
// SMOOTH SCROLL
// ═══════════════════════════════════════════════════════════
const OperanSmoothScroll = {
  init: function() {
    document.querySelectorAll('a[href^="#"]').forEach(function(a) {
      a.addEventListener('click', function(e) {
        var href = this.getAttribute('href');
        if (href === '#') return;
        var target = document.querySelector(href);
        if (target) { e.preventDefault(); target.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
      });
    });
  }
};

// ═══════════════════════════════════════════════════════════
// FLOATING DOTS
// ═══════════════════════════════════════════════════════════
const OperanFloatingDots = {
  init: function(containerId) {
    var container = document.getElementById(containerId);
    if (!container) return;
    for (var i = 0; i < 15; i++) {
      var d = document.createElement('div');
      d.className = 'floating-dot';
      d.style.left = Math.random()*100+'%';
      d.style.animationDuration = (8+Math.random()*12)+'s';
      d.style.animationDelay = (Math.random()*10)+'s';
      d.style.width = d.style.height = (1+Math.random()*2)+'px';
      container.appendChild(d);
    }
  }
};

// ═══════════════════════════════════════════════════════════
// HERO PARALLAX LAYERS
// ═══════════════════════════════════════════════════════════
const OperanParallax = {
  init: function() {
    var layers = document.querySelectorAll('[data-parallax]');
    if (layers.length === 0) return;
    var ticking = false;
    window.addEventListener('scroll', function() {
      if (!ticking) {
        requestAnimationFrame(function() {
          var scrollY = window.scrollY;
          layers.forEach(function(layer) {
            var speed = parseFloat(layer.dataset.parallax);
            layer.style.transform = 'translateY(' + (scrollY * speed) + 'px)';
          });
          ticking = false;
        });
        ticking = true;
      }
    });
  }
};

// ═══════════════════════════════════════════════════════════
// RAIN ANIMATION (shared, all pages)
// ═══════════════════════════════════════════════════════════
const OperanRain = {
  canvas: null, ctx: null, w: 0, h: 0,
  drops: [], raf: null, active: false,

  init(canvasId) {
    this.canvas = document.getElementById(canvasId);
    if (!this.canvas) return;
    this.ctx = this.canvas.getContext('2d');
    this.active = true;
    this.resize();
    this.initDrops();
    this.animate();
  },

  resize() {
    this.w = this.canvas.width = window.innerWidth;
    this.h = this.canvas.height = window.innerHeight;
    this.initDrops();
  },

  initDrops() {
    var count = Math.floor(this.w / 4);
    this.drops = [];
    for (var i = 0; i < count; i++) {
      this.drops.push({
        x: Math.random() * this.w,
        y: Math.random() * this.h,
        speed: 2 + Math.random() * 6,
        length: 8 + Math.random() * 20,
        opacity: 0.05 + Math.random() * 0.15
      });
    }
  },

  animate() {
    if (!this.active) return;
    this.raf = requestAnimationFrame(this.animate.bind(this));
    this.ctx.clearRect(0, 0, this.w, this.h);
    for (var i = 0; i < this.drops.length; i++) {
      var d = this.drops[i];
      this.ctx.beginPath();
      this.ctx.moveTo(d.x, d.y);
      this.ctx.lineTo(d.x + 0.5, d.y + d.length);
      this.ctx.strokeStyle = 'rgba(100, 160, 200, ' + d.opacity + ')';
      this.ctx.lineWidth = 0.8;
      this.ctx.stroke();
      d.y += d.speed;
      if (d.y > this.h) {
        d.y = -d.length;
        d.x = Math.random() * this.w;
      }
    }
  },

  stop() {
    this.active = false;
    if (this.raf) cancelAnimationFrame(this.raf);
  }
};

// ═══════════════════════════════════════════════════════════
// HERO SLIDESHOW (shared, all pages)
// ═══════════════════════════════════════════════════════════
const OperanSlideshow = {
  container: null, slides: [], dots: [], current: 0, timer: null, fading: false,
  init(containerId, interval) {
    this.container = document.getElementById(containerId);
    if (!this.container) return;
    this.slides = this.container.querySelectorAll('.slide');
    this.dots = document.querySelectorAll('.slide-dot');
    if (this.slides.length === 0) return;
    var iv = interval || 5000;
    this.slides[0].classList.add('active');
    if (this.dots.length) this.dots[0].classList.add('active');
    this.cycle(iv);
  },
  cycle: function(iv) {
    var self = this;
    this.timer = setInterval(function() { self.next(); }, iv);
  },
  next: function() {
    if (this.fading) return;
    this.fading = true;
    var cur = this.slides[this.current];
    if (this.dots[this.current]) this.dots[this.current].classList.remove('active');
    this.current = (this.current + 1) % this.slides.length;
    var nxt = this.slides[this.current];
    cur.classList.remove('active');
    var self = this;
    setTimeout(function() {
      nxt.classList.add('active');
      if (self.dots[self.current]) self.dots[self.current].classList.add('active');
      self.fading = false;
    }, 800);
  },
  pause: function() { clearInterval(this.timer); },
  resume: function(iv) { this.cycle(iv); }
};

// ═══════════════════════════════════════════════════════════
// INIT ON DOM READY
// ═══════════════════════════════════════════════════════════
document.addEventListener('DOMContentLoaded', function() {
  OperanNav.init();
  OperanReveal.init();
  OperanCounters.init();
  Operan3DCards.init('.tilt-card');
  OperanTabs.init('industryTabs');
  OperanCursorGlow.init();
  OperanSmoothScroll.init();
  OperanFloatingDots.init('floating-dots');
  OperanParallax.init();

  // Rain on all pages
  var rainCanvas = document.getElementById('rain-canvas');
  if (rainCanvas) {
    OperanRain.init('rain-canvas');
    rainCanvas.classList.add('active');
  }

  // Real-time event stream (index only, if canvas exists)
  if (document.getElementById('webgl-canvas')) {
    OperanDataVisual.init('webgl-canvas', {
      rowCount: 5,
      eventCount: 25
    });
  }

  // Slideshow
  if (document.getElementById('hero-slider')) {
    OperanSlideshow.init('hero-slider', 5000);
  }

  window.addEventListener('resize', function() {
    if (OperanRain.canvas) OperanRain.resize();
  });
});