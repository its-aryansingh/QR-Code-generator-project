import type { Metadata } from 'next';
import { QRTypePage } from '@/components/seo/QRTypePage';

export const metadata: Metadata = {
  title: 'Free Social Media QR Code Generator | QRit',
  description: 'Create a QR code for your Instagram, Twitter, Facebook, or custom social profile. Grow your followers offline for free.',
  keywords: ['social media qr code', 'instagram qr code generator', 'facebook qr code', 'qr code for tiktok'],
  alternates: {
    canonical: 'https://qrit.com/social-media-qr-code'
  }
};

export default function SocialMediaQRPage() {
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "Social Media QR Code Generator - QRit",
    "operatingSystem": "Web",
    "applicationCategory": "SocialNetworkingApplication",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "USD"
    }
  };

  const howToSteps = [
    { title: "Select Platform", desc: "Choose Instagram, Twitter/X, TikTok, LinkedIn, YouTube, or Custom." },
    { title: "Enter Username", desc: "Type in your social media handle or profile username." },
    { title: "Download QR", desc: "Your QR code is instantly ready to be printed on your marketing materials." }
  ];

  const useCases = [
    { title: "Retail Stores & Salons", desc: "Place a QR code at the checkout counter asking happy customers to follow your Instagram." },
    { title: "Influencer Networking", desc: "Share your TikTok or YouTube channel easily with brands and peers." },
    { title: "Print Advertising", desc: "Add your Facebook page QR code to flyers and brochures to build a local community." },
    { title: "Product Packaging", desc: "Encourage buyers to tag your products on social media by linking to your profile." }
  ];

  const faqs = [
    { question: "How does it work?", answer: "The generator creates a standard URL that directs straight to your social profile. For example, selecting Instagram and typing 'QRit' makes a code for 'https://instagram.com/QRit'." },
    { question: "Will it open the social media app?", answer: "Yes, on most modern smartphones, scanning the link will prompt the device to open the native app (like the Instagram app) if it's installed, rather than the web browser." },
    { question: "Can I link to a specific post?", answer: "To link to a specific post or video rather than a profile, select 'Custom URL' or use our standard URL QR Code Generator and paste the full link." },
    { question: "Is this free to use?", answer: "Yes, creating static social media QR codes is 100% free with QRit." },
    { question: "Can I link all my profiles in one code?", answer: "To link multiple profiles in one scan, use our 'Multi-Link' or link-in-bio QR code feature available in our generator." }
  ];

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <QRTypePage
        type="social"
        heroH1="Free Social Media QR Code Generator"
        heroSubtitle="Connect your physical audience to your digital profiles. Grow your followers on Instagram, TikTok, Twitter, and more."
        howToSteps={howToSteps}
        useCases={useCases}
        faqs={faqs}
      />
    </>
  );
}
