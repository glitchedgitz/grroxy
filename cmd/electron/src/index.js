// This file is the entry point for the Electron application.

const { app, BrowserWindow, ipcMain, nativeImage, shell } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const net = require('net')

let mainWindow = null
let grroxyProcess = null

const platformMap = { darwin: 'mac', win32: 'win', linux: 'linux' }

function getBinDir() {
    if (app.isPackaged) {
        return path.join(process.resourcesPath, 'bin')
    }
    const plat = platformMap[process.platform] || process.platform
    return path.join(__dirname, '..', 'bin', plat, process.arch)
}

function getBinPath(name) {
    const ext = process.platform === 'win32' ? '.exe' : ''
    return path.join(getBinDir(), name + ext)
}

function findAvailablePort(startPort) {
    return new Promise((resolve, reject) => {
        const server = net.createServer()
        server.listen(startPort, '127.0.0.1', () => {
            const port = server.address().port
            server.close(() => resolve(port))
        })
        server.on('error', () => {
            // Port in use, try next
            resolve(findAvailablePort(startPort + 1))
        })
    })
}

function startGrroxy(host) {
    const binDir = getBinDir()
    const grroxyPath = getBinPath('grroxy')

    const env = {
        ...process.env,
        PATH: binDir + path.delimiter + (process.env.PATH || '')
    }

    grroxyProcess = spawn(grroxyPath, ['start', '--host', host], {
        stdio: 'pipe',
        env: env,
        detached: true,
    })

    grroxyProcess.stdout.on('data', (data) => {
        console.log(`[grroxy] ${data.toString().trimEnd()}`)
    })

    grroxyProcess.stderr.on('data', (data) => {
        console.error(`[grroxy] ${data.toString().trimEnd()}`)
    })

    grroxyProcess.on('error', (err) => {
        console.error(`[grroxy] Failed to start: ${err.message}`)
        grroxyProcess = null
    })

    grroxyProcess.on('close', (code) => {
        console.log(`[grroxy] Process exited with code ${code}`)
        grroxyProcess = null
    })
}

function stopGrroxy() {
    if (grroxyProcess) {
        const pid = grroxyProcess.pid
        grroxyProcess = null
        // Kill the entire process group (grroxy + grroxy-app + grroxy-tool)
        try { process.kill(-pid, 'SIGTERM') } catch { /* already dead */ }
    }
}

function createWindow(grroxyURL) {
    const iconPath = path.resolve(__dirname, "icons", "grroxy.png")

    // Windows-specific: use frameless window for custom titlebar
    const isWindows = process.platform === 'win32';

    mainWindow = new BrowserWindow({
        width: 1080,
        height: 720,
        frame: !isWindows,
        autoHideMenuBar: true,

        icon: iconPath,

        titleBarStyle: isWindows ? undefined : 'hiddenInset',
        title: 'Grroxy',

        titleBarOverlay: isWindows ? undefined : {
            color: '#00000000',
            symbolColor: '#FFFFFF',
        },

        vibrancy: isWindows ? undefined : 'under-window',

        webPreferences: {
            preload: path.join(__dirname, 'preload.js'),
            contextIsolation: true,
            nodeIntegration: false,
        }
    })

    mainWindow.loadURL(grroxyURL)
    // mainWindow.loadURL('http://localhost:5173')

    mainWindow.on('enter-full-screen', () => {
        console.log('[main] Entered fullscreen');
        mainWindow.webContents.send('fullscreen-changed', true);
    });

    mainWindow.on('leave-full-screen', () => {
        console.log('[main] Left fullscreen');
        mainWindow.webContents.send('fullscreen-changed', false);
    });

    if (isWindows) {
        mainWindow.on('maximize', () => {
            mainWindow.webContents.send('window-maximized', true);
        });

        mainWindow.on('unmaximize', () => {
            mainWindow.webContents.send('window-maximized', false);
        });
    }

    if (process.platform === 'darwin') {
        app.dock.setIcon(nativeImage.createFromPath(iconPath))
    }
}

let grroxyURL = null

app.whenReady()
    .then(async () => {
        const port = await findAvailablePort(8090)
        const host = `127.0.0.1:${port}`
        grroxyURL = `http://${host}`

        console.log(`[electron] Starting grroxy on ${host}`)
        startGrroxy(host)

        // Register IPC handlers
        ipcMain.handle('check-fullscreen', (event) => {
            if (mainWindow) {
                const isFs = mainWindow.isFullScreen();
                console.log('[main] check-fullscreen →', isFs);
                return isFs;
            }
            return false;
        });

        ipcMain.handle('window-minimize', (event) => {
            if (mainWindow) mainWindow.minimize();
        });

        ipcMain.handle('window-maximize', (event) => {
            if (mainWindow) {
                if (mainWindow.isMaximized()) mainWindow.unmaximize();
                else mainWindow.maximize();
            }
        });

        ipcMain.handle('window-close', (event) => {
            if (mainWindow) mainWindow.close();
        });

        ipcMain.handle('window-is-maximized', (event) => {
            if (mainWindow) return mainWindow.isMaximized();
            return false;
        });

        ipcMain.handle('get-version', () => {
            return { version: app.getVersion() }
        });

        ipcMain.handle('open-url', (event, url) => {
            shell.openExternal(url)
        });

        createWindow(grroxyURL)

        app.on('activate', function () {
            if (BrowserWindow.getAllWindows().length === 0) createWindow(grroxyURL)
        })
    })

app.on('before-quit', () => {
    stopGrroxy()
})

app.on('window-all-closed', function () {
    if (process.platform !== 'darwin') app.quit()
})
