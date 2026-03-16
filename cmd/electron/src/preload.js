const { contextBridge, ipcRenderer } = require('electron');

console.log('[preload] Executed preload.js');

contextBridge.exposeInMainWorld('electronAPI', {
    onFullscreenChange: (callback) => {
        console.log('[preload] Setting up fullscreen listener');
        ipcRenderer.on('fullscreen-changed', (event, isFullscreen) => {
            console.log('[preload] fullscreen-changed received:', isFullscreen);
            callback(isFullscreen);
        });
    },
    isFullscreen: async () => {
        console.log('[preload] Invoking check-fullscreen');
        const result = await ipcRenderer.invoke('check-fullscreen');
        console.log('[preload] check-fullscreen returned:', result);
        return result;
    },
    // Window control functions for custom titlebar (Windows)
    windowMinimize: () => {
        ipcRenderer.invoke('window-minimize');
    },
    windowMaximize: () => {
        ipcRenderer.invoke('window-maximize');
    },
    windowClose: () => {
        ipcRenderer.invoke('window-close');
    },
    windowIsMaximized: async () => {
        return await ipcRenderer.invoke('window-is-maximized');
    },
    onWindowMaximized: (callback) => {
        ipcRenderer.on('window-maximized', (event, isMaximized) => {
            callback(isMaximized);
        });
    },

    // App info
    getVersion: () => ipcRenderer.invoke('get-version'),
    openURL: (url) => ipcRenderer.invoke('open-url', url),
    getHost: () => ipcRenderer.invoke('get-host'),
});
