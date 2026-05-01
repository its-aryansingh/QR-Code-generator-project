import React, { useState, useEffect } from "react";
import { countryCodes } from "@/lib/countryCodes";
import { QRIcons } from "@/lib/qr-icons"; // Assuming icons are here or need to be moved

// Interface for all potential props - adhering to existing types in page.tsx
// Helper component for Phone Input with Country Code
const PhoneInput = ({
    value,
    onChange,
    placeholder = "98765 43210",
    label = "Phone Number"
}: {
    value: string,
    onChange: (val: string) => void,
    placeholder?: string,
    label?: string
}) => {
    const [selectedCode, setSelectedCode] = useState("+91");
    const [number, setNumber] = useState("");

    useEffect(() => {
        if (!value) {
            setNumber("");
            return;
        }

        const foundCode = countryCodes.find(c => value.trim().startsWith(c.code));
        if (foundCode) {
            setSelectedCode(foundCode.code);
            setNumber(value.slice(foundCode.code.length).trim());
        } else {
            // Fallback: try to keep existing value in number if it doesnt match formatting
            // or if it's just a raw number without code.
            setNumber(value);
        }
    }, [value]);

    const handleCodeChange = (code: string) => {
        setSelectedCode(code);
        onChange(`${code} ${number}`);
    };

    const handleNumberChange = (num: string) => {
        setNumber(num);
        onChange(`${selectedCode} ${num}`);
    };

    return (
        <div>
            {label && <label className="block text-sm font-medium text-slate-300 mb-2">{label}</label>}
            <div className="flex gap-2">
                <select
                    value={selectedCode}
                    onChange={(e) => handleCodeChange(e.target.value)}
                    className="w-24 px-2 py-3.5 bg-white/5 border border-white/10 rounded-xl text-white focus:outline-none focus:border-violet-500/50 focus:ring-2 focus:ring-violet-500/20 transition-all duration-300 appearance-none cursor-pointer"
                >
                    {countryCodes.map((c) => (
                        <option key={c.code} value={c.code} className="bg-[#12121a] text-white">
                            {c.code} ({c.country})
                        </option>
                    ))}
                </select>
                <input
                    type="tel"
                    value={number}
                    onChange={(e) => handleNumberChange(e.target.value)}
                    placeholder={placeholder}
                    className="flex-1 px-4 py-3.5 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-slate-500 focus:outline-none focus:border-violet-500/50 focus:ring-2 focus:ring-violet-500/20 transition-all duration-300"
                />
            </div>
        </div>
    );
};

interface CreateQRPanelProps {
    selectedType: string;
    setSelectedType: (type: string) => void;
    isDynamicQR: boolean;
    setIsDynamicQR: (isDynamic: boolean) => void;
    content: string;
    setContent: (content: string) => void;
    vcardData: any;
    setVcardData: (data: any) => void;
    emailData: any;
    setEmailData: (data: any) => void;
    smsData: any;
    setSmsData: (data: any) => void;
    wifiData: any;
    setWifiData: (data: any) => void;
    locationData: any;
    setLocationData: (data: any) => void;
    eventData: any;
    setEventData: (data: any) => void;
    socialData: any;
    setSocialData: (data: any) => void;
    bitcoinData: any;
    setBitcoinData: (data: any) => void;
    upiData: any;
    setUpiData: (data: any) => void;
    whatsappData: any;
    setWhatsappData: (data: any) => void;
    mecardData: any;
    setMecardData: (data: any) => void;
    multiLinkData: any[];
    setMultiLinkData: (data: any[]) => void;
}

export const CreateQRPanel: React.FC<CreateQRPanelProps> = ({
    selectedType,
    setSelectedType,
    isDynamicQR,
    setIsDynamicQR,
    content,
    setContent,
    vcardData,
    setVcardData,
    emailData,
    setEmailData,
    smsData,
    setSmsData,
    wifiData,
    setWifiData,
    locationData,
    setLocationData,
    eventData,
    setEventData,
    socialData,
    setSocialData,
    bitcoinData,
    setBitcoinData,
    upiData,
    setUpiData,
    whatsappData,
    setWhatsappData,
    mecardData,
    setMecardData,
    multiLinkData,
    setMultiLinkData,
}) => {
    const inputClass = "w-full px-4 py-3.5 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-slate-500 focus:outline-none focus:border-violet-500/50 focus:ring-2 focus:ring-violet-500/20 transition-all duration-300";

    return (
        <div className="space-y-5">
            {/* URL Input */}
            {selectedType === "url" && (
                <div className="animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <label className="block text-sm font-medium text-slate-300 mb-2">Website URL</label>
                    <input
                        type="url"
                        value={content}
                        onChange={(e) => setContent(e.target.value)}
                        placeholder="https://yourwebsite.com"
                        className={inputClass}
                    />
                    <p className="mt-2 text-xs text-slate-500">Enter any valid URL to generate a scannable QR code</p>
                </div>
            )}

            {/* Text Input */}
            {selectedType === "text" && (
                <div className="animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <label className="block text-sm font-medium text-slate-300 mb-2">Your Message</label>
                    <textarea
                        value={content}
                        onChange={(e) => setContent(e.target.value)}
                        placeholder="Enter your text message here..."
                        rows={4}
                        className={`${inputClass} resize-none`}
                    />
                </div>
            )}

            {/* Phone Input */}
            {selectedType === "phone" && (
                <div className="animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <PhoneInput
                        value={content}
                        onChange={setContent}
                        label="Phone Number"
                    />
                </div>
            )}

            {/* vCard Form */}
            {selectedType === "vcard" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Full Name *</label>
                        <input
                            type="text"
                            value={vcardData.name}
                            onChange={(e) => setVcardData({ ...vcardData, name: e.target.value })}
                            placeholder="John Doe"
                            className={inputClass}
                        />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <PhoneInput
                                value={vcardData.phone}
                                onChange={(val) => setVcardData({ ...vcardData, phone: val })}
                                label="Phone"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Email</label>
                            <input
                                type="email"
                                value={vcardData.email}
                                onChange={(e) => setVcardData({ ...vcardData, email: e.target.value })}
                                placeholder="john@company.com"
                                className={inputClass}
                            />
                        </div>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Company</label>
                        <input
                            type="text"
                            value={vcardData.company}
                            onChange={(e) => setVcardData({ ...vcardData, company: e.target.value })}
                            placeholder="Acme Inc."
                            className={inputClass}
                        />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Job Title</label>
                            <input
                                type="text"
                                value={vcardData.title || ""}
                                onChange={(e) => setVcardData({ ...vcardData, title: e.target.value })}
                                placeholder="Software Engineer"
                                className={inputClass}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Website</label>
                            <input
                                type="url"
                                value={vcardData.url || ""}
                                onChange={(e) => setVcardData({ ...vcardData, url: e.target.value })}
                                placeholder="https://example.com"
                                className={inputClass}
                            />
                        </div>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Address</label>
                        <input
                            type="text"
                            value={vcardData.address || ""}
                            onChange={(e) => setVcardData({ ...vcardData, address: e.target.value })}
                            placeholder="123 Main St, City, Country"
                            className={inputClass}
                        />
                    </div>
                </div>
            )}

            {/* Email Form */}
            {selectedType === "email" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Recipient Email *</label>
                        <input
                            type="email"
                            value={emailData.to}
                            onChange={(e) => setEmailData({ ...emailData, to: e.target.value })}
                            placeholder="recipient@email.com"
                            className={inputClass}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Subject</label>
                        <input
                            type="text"
                            value={emailData.subject}
                            onChange={(e) => setEmailData({ ...emailData, subject: e.target.value })}
                            placeholder="Email subject line"
                            className={inputClass}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Message Body</label>
                        <textarea
                            value={emailData.body}
                            onChange={(e) => setEmailData({ ...emailData, body: e.target.value })}
                            placeholder="Your message..."
                            rows={3}
                            className={`${inputClass} resize-none`}
                        />
                    </div>
                </div>
            )}

            {/* SMS Form */}
            {selectedType === "sms" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <PhoneInput
                            value={smsData.phone}
                            onChange={(val) => setSmsData({ ...smsData, phone: val })}
                            label="Phone Number *"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Pre-filled Message</label>
                        <textarea
                            value={smsData.message}
                            onChange={(e) => setSmsData({ ...smsData, message: e.target.value })}
                            placeholder="Optional message to pre-fill..."
                            rows={3}
                            className={`${inputClass} resize-none`}
                        />
                    </div>
                </div>
            )}

            {/* WiFi Form */}
            {selectedType === "wifi" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Network Name (SSID) *</label>
                        <input
                            type="text"
                            value={wifiData.ssid}
                            onChange={(e) => setWifiData({ ...wifiData, ssid: e.target.value })}
                            placeholder="Your WiFi Network"
                            className={inputClass}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Password</label>
                        <input
                            type="password"
                            value={wifiData.password}
                            onChange={(e) => setWifiData({ ...wifiData, password: e.target.value })}
                            placeholder="WiFi password"
                            className={inputClass}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Security</label>
                        <select
                            value={wifiData.encryption}
                            onChange={(e) => setWifiData({ ...wifiData, encryption: e.target.value })}
                            className={inputClass}
                        >
                            <option value="WPA">WPA/WPA2</option>
                            <option value="WEP">WEP</option>
                            <option value="">None</option>
                        </select>
                    </div>
                </div>
            )}

            {/* Location Form */}
            {selectedType === "location" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Latitude *</label>
                            <input
                                type="text"
                                value={locationData.lat}
                                onChange={(e) => setLocationData({ ...locationData, lat: e.target.value })}
                                placeholder="19.0760"
                                className={inputClass}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Longitude *</label>
                            <input
                                type="text"
                                value={locationData.lng}
                                onChange={(e) => setLocationData({ ...locationData, lng: e.target.value })}
                                placeholder="72.8777"
                                className={inputClass}
                            />
                        </div>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Place Name (Optional)</label>
                        <input
                            type="text"
                            value={locationData.name}
                            onChange={(e) => setLocationData({ ...locationData, name: e.target.value })}
                            placeholder="Mumbai, Maharashtra"
                            className={inputClass}
                        />
                    </div>
                </div>
            )}

            {/* Event Form */}
            {selectedType === "event" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Event Title *</label>
                        <input
                            type="text"
                            value={eventData.title}
                            onChange={(e) => setEventData({ ...eventData, title: e.target.value })}
                            placeholder="Team Meeting"
                            className={inputClass}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Location</label>
                        <input
                            type="text"
                            value={eventData.location}
                            onChange={(e) => setEventData({ ...eventData, location: e.target.value })}
                            placeholder="Conference Room A"
                            className={inputClass}
                        />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Start Date/Time *</label>
                            <input
                                type="datetime-local"
                                value={eventData.startDate}
                                onChange={(e) => setEventData({ ...eventData, startDate: e.target.value })}
                                className={inputClass}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">End Date/Time</label>
                            <input
                                type="datetime-local"
                                value={eventData.endDate}
                                onChange={(e) => setEventData({ ...eventData, endDate: e.target.value })}
                                className={inputClass}
                            />
                        </div>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Description</label>
                        <textarea
                            value={eventData.description}
                            onChange={(e) => setEventData({ ...eventData, description: e.target.value })}
                            placeholder="Event details..."
                            rows={2}
                            className={`${inputClass} resize-none`}
                        />
                    </div>
                </div>
            )}

            {/* Social Media Form */}
            {selectedType === "social" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Platform</label>
                        <select
                            value={socialData.platform}
                            onChange={(e) => setSocialData({ ...socialData, platform: e.target.value })}
                            className={inputClass}
                        >
                            <option value="instagram" className="bg-[#12121a] text-white">Instagram</option>
                            <option value="twitter" className="bg-[#12121a] text-white">Twitter/X</option>
                            <option value="facebook" className="bg-[#12121a] text-white">Facebook</option>
                            <option value="linkedin" className="bg-[#12121a] text-white">LinkedIn</option>
                            <option value="tiktok" className="bg-[#12121a] text-white">TikTok</option>
                            <option value="youtube" className="bg-[#12121a] text-white">YouTube</option>
                            <option value="custom" className="bg-[#12121a] text-white">Custom URL</option>
                        </select>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Username *</label>
                        <input
                            type="text"
                            value={socialData.username}
                            onChange={(e) => setSocialData({ ...socialData, username: e.target.value })}
                            placeholder="your_username"
                            className={inputClass}
                        />
                    </div>
                </div>
            )}

            {/* Bitcoin Form */}
            {selectedType === "bitcoin" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Bitcoin Address *</label>
                        <input
                            type="text"
                            value={bitcoinData.address}
                            onChange={(e) => setBitcoinData({ ...bitcoinData, address: e.target.value })}
                            placeholder="bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
                            className={inputClass}
                        />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Amount (BTC)</label>
                            <input
                                type="text"
                                value={bitcoinData.amount}
                                onChange={(e) => setBitcoinData({ ...bitcoinData, amount: e.target.value })}
                                placeholder="0.001"
                                className={inputClass}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Label</label>
                            <input
                                type="text"
                                value={bitcoinData.label}
                                onChange={(e) => setBitcoinData({ ...bitcoinData, label: e.target.value })}
                                placeholder="Payment for..."
                                className={inputClass}
                            />
                        </div>
                    </div>
                </div>
            )}

            {/* UPI Payment Form */}
            {selectedType === "upi" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">UPI ID / VPA *</label>
                        <input
                            type="text"
                            value={upiData.upiId}
                            onChange={(e) => setUpiData({ ...upiData, upiId: e.target.value })}
                            placeholder="yourname@okicici, mobile@paytm"
                            className={inputClass}
                        />
                        <p className="mt-1.5 text-xs text-slate-500">Your Virtual Payment Address (e.g. name@bankname, phone@upi)</p>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Payee Name</label>
                        <input
                            type="text"
                            value={upiData.name}
                            onChange={(e) => setUpiData({ ...upiData, name: e.target.value })}
                            placeholder="Your Name or Business"
                            className={inputClass}
                        />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Amount (₹)</label>
                            <input
                                type="number"
                                value={upiData.amount}
                                onChange={(e) => setUpiData({ ...upiData, amount: e.target.value })}
                                placeholder="100.00"
                                className={inputClass}
                                min="0"
                                step="0.01"
                            />
                            <p className="mt-1.5 text-xs text-slate-500">Leave empty to let payer enter amount</p>
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Reference ID</label>
                            <input
                                type="text"
                                value={upiData.refId}
                                onChange={(e) => setUpiData({ ...upiData, refId: e.target.value })}
                                placeholder="ORDER-123"
                                className={inputClass}
                            />
                        </div>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Transaction Note</label>
                        <input
                            type="text"
                            value={upiData.note}
                            onChange={(e) => setUpiData({ ...upiData, note: e.target.value })}
                            placeholder="Payment for groceries, Subscription fee..."
                            className={inputClass}
                        />
                    </div>
                </div>
            )}

            {/* WhatsApp Form */}
            {selectedType === "whatsapp" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <PhoneInput
                            value={whatsappData.phone}
                            onChange={(val) => setWhatsappData({ ...whatsappData, phone: val })}
                            label="Phone Number *"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Message</label>
                        <textarea
                            value={whatsappData.message}
                            onChange={(e) => setWhatsappData({ ...whatsappData, message: e.target.value })}
                            placeholder="Hello, I'm interested..."
                            className={`${inputClass} min-h-[100px] resize-none`}
                        />
                    </div>
                </div>
            )}

            {/* MeCard Form */}
            {selectedType === "mecard" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Name *</label>
                        <input
                            type="text"
                            value={mecardData.name}
                            onChange={(e) => setMecardData({ ...mecardData, name: e.target.value })}
                            placeholder="John Doe"
                            className={inputClass}
                        />
                    </div>
                    {/* Multiple Phones */}
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Phone Numbers</label>
                        {mecardData.phones.map((phone: string, i: number) => (
                            <div key={i} className="flex gap-2 mb-2 items-start">
                                <div className="flex-1">
                                    <PhoneInput
                                        value={phone}
                                        onChange={(val) => {
                                            const newPhones = [...mecardData.phones];
                                            newPhones[i] = val;
                                            setMecardData({ ...mecardData, phones: newPhones });
                                        }}
                                        label=""
                                        placeholder="98765 43210"
                                    />
                                </div>
                                {mecardData.phones.length > 1 && (
                                    <button
                                        type="button"
                                        onClick={() => {
                                            const newPhones = mecardData.phones.filter((_: any, idx: number) => idx !== i);
                                            setMecardData({ ...mecardData, phones: newPhones });
                                        }}
                                        className="px-3 py-2 rounded-lg bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-all text-sm"
                                    >✕</button>
                                )}
                            </div>
                        ))}
                        <button
                            type="button"
                            onClick={() => setMecardData({ ...mecardData, phones: [...mecardData.phones, "+91 "] })}
                            className="text-xs text-violet-400 hover:text-violet-300 transition-colors flex items-center gap-1 mt-1"
                        >
                            <span className="text-lg leading-none">+</span> Add Phone
                        </button>
                    </div>
                    {/* Multiple Emails */}
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Email Addresses</label>
                        {mecardData.emails.map((email: string, i: number) => (
                            <div key={i} className="flex gap-2 mb-2">
                                <input
                                    type="email"
                                    value={email}
                                    onChange={(e) => {
                                        const newEmails = [...mecardData.emails];
                                        newEmails[i] = e.target.value;
                                        setMecardData({ ...mecardData, emails: newEmails });
                                    }}
                                    placeholder="john@example.com"
                                    className={`${inputClass} flex-1`}
                                />
                                {mecardData.emails.length > 1 && (
                                    <button
                                        type="button"
                                        onClick={() => {
                                            const newEmails = mecardData.emails.filter((_: any, idx: number) => idx !== i);
                                            setMecardData({ ...mecardData, emails: newEmails });
                                        }}
                                        className="px-3 py-2 rounded-lg bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-all text-sm"
                                    >✕</button>
                                )}
                            </div>
                        ))}
                        <button
                            type="button"
                            onClick={() => setMecardData({ ...mecardData, emails: [...mecardData.emails, ""] })}
                            className="text-xs text-violet-400 hover:text-violet-300 transition-colors flex items-center gap-1 mt-1"
                        >
                            <span className="text-lg leading-none">+</span> Add Email
                        </button>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-slate-300 mb-2">Address</label>
                        <input
                            type="text"
                            value={mecardData.address}
                            onChange={(e) => setMecardData({ ...mecardData, address: e.target.value })}
                            placeholder="123 Main St, City"
                            className={inputClass}
                        />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Website</label>
                            <input
                                type="url"
                                value={mecardData.url}
                                onChange={(e) => setMecardData({ ...mecardData, url: e.target.value })}
                                placeholder="https://example.com"
                                className={inputClass}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-slate-300 mb-2">Note</label>
                            <input
                                type="text"
                                value={mecardData.note}
                                onChange={(e) => setMecardData({ ...mecardData, note: e.target.value })}
                                placeholder="Memo"
                                className={inputClass}
                            />
                        </div>
                    </div>
                </div>
            )}

            {/* Multi-Link Form */}
            {selectedType === "multilink" && (
                <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                    <label className="block text-sm font-medium text-slate-300 mb-2">Links</label>
                    {multiLinkData.map((link, i) => (
                        <div key={i} className="flex gap-2 items-start">
                            <input
                                type="text"
                                value={link.label}
                                onChange={(e) => {
                                    const newLinks = [...multiLinkData];
                                    newLinks[i] = { ...newLinks[i], label: e.target.value };
                                    setMultiLinkData(newLinks);
                                }}
                                placeholder="Label (optional)"
                                className={`${inputClass} w-1/3`}
                            />
                            <input
                                type="url"
                                value={link.url}
                                onChange={(e) => {
                                    const newLinks = [...multiLinkData];
                                    newLinks[i] = { ...newLinks[i], url: e.target.value };
                                    setMultiLinkData(newLinks);
                                }}
                                placeholder="https://example.com"
                                className={`${inputClass} flex-1`}
                            />
                            {multiLinkData.length > 1 && (
                                <button
                                    type="button"
                                    onClick={() => setMultiLinkData(multiLinkData.filter((_, idx) => idx !== i))}
                                    className="px-3 py-2 rounded-lg bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-all text-sm mt-0.5"
                                >✕</button>
                            )}
                        </div>
                    ))}
                    <button
                        type="button"
                        onClick={() => setMultiLinkData([...multiLinkData, { label: "", url: "" }])}
                        className="text-xs text-violet-400 hover:text-violet-300 transition-colors flex items-center gap-1 mt-1"
                    >
                        <span className="text-lg leading-none">+</span> Add Link
                    </button>
                </div>
            )}
        </div>
    );
};
